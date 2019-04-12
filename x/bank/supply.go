package bank

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Supplier represents the keeps track of the total supply amounts in the network
type Supplier struct {
	CirculatingSupply sdk.Coins `json:"circulating_supply"` // supply held by accounts that's not vesting
	VestingSupply     sdk.Coins `json:"vesting_supply"`     // locked supply held by vesting accounts
	HoldersSupply     sdk.Coins `json:"holders_supply"`     // supply held by non acccount token holders (e.g modules)
	TotalSupply       sdk.Coins `json:"total_supply"`       // total supply of the network
}

// CirculatingAmountOf returns the circulating supply of a coin denomination
func (supplier Supplier) CirculatingAmountOf(denom string) sdk.Int {
	return supplier.CirculatingSupply.AmountOf(denom)
}

// VestingAmountOf returns the vesting supply of a coin denomination
func (supplier Supplier) VestingAmountOf(denom string) sdk.Int {
	return supplier.VestingSupply.AmountOf(denom)
}

// HoldersAmountOf returns the total token holders' supply of a coin denomination
func (supplier Supplier) HoldersAmountOf(denom string) sdk.Int {
	return supplier.HoldersSupply.AmountOf(denom)
}

// TotalAmountOf returns the total supply of a coin denomination
func (supplier Supplier) TotalAmountOf(denom string) sdk.Int {
	return supplier.TotalSupply.AmountOf(denom)
}

// GetSupplier retrieves the Supplier from store
func (keeper BaseKeeper) GetSupplier(ctx sdk.Context) (supplier Supplier) {
	store := ctx.KVStore(keeper.storeKey)
	b := store.Get(supplierKey)
	if b == nil {
		panic("Stored fee pool should not have been nil")
	}
	keeper.cdc.MustUnmarshalBinaryLengthPrefixed(b, &supplier)
	return
}

// SetSupplier sets the Supplier to store
func (keeper BaseKeeper) SetSupplier(ctx sdk.Context, supplier Supplier) {
	store := ctx.KVStore(keeper.storeKey)
	b := keeper.cdc.MustMarshalBinaryLengthPrefixed(supplier)
	store.Set(supplierKey, b)
}

// GetTokenHolders returns all the token holders
func (keeper BaseKeeper) GetTokenHolders(ctx sdk.Context) (
	tokenHolders []TokenHolder, err error) {
	store := ctx.KVStore(keeper.storeKey)
	iterator := sdk.KVStorePrefixIterator(store, holderKeyPrefix)
	defer iterator.Close()

	var tokenHolder TokenHolder
	for ; iterator.Valid(); iterator.Next() {
		err = keeper.cdc.UnmarshalBinaryLengthPrefixed(iterator.Value(), &tokenHolder)
		if err != nil {
			return
		}
		tokenHolders = append(tokenHolders, tokenHolder)
	}
	return
}

// GetTokenHolder returns a token holder instance
func (keeper BaseKeeper) GetTokenHolder(ctx sdk.Context, moduleName string) (
	tokenHolder TokenHolder, err error) {
	store := ctx.KVStore(keeper.storeKey)
	b := store.Get(GetTokenHolderKey(moduleName))
	if b == nil {
		err = fmt.Errorf("module %s doesn't exist", moduleName)
		return
	}
	keeper.cdc.MustUnmarshalBinaryLengthPrefixed(b, tokenHolder)
	return
}

// SetTokenHolder sets a holder to store
func (keeper BaseKeeper) SetTokenHolder(ctx sdk.Context, tokenHolder TokenHolder) {
	store := ctx.KVStore(keeper.storeKey)
	holderKey := GetTokenHolderKey(tokenHolder.GetModuleName())
	b := keeper.cdc.MustMarshalBinaryLengthPrefixed(tokenHolder)
	store.Set(holderKey, b)
}

// InflateSupply adds tokens to the circulating supply
func (keeper BaseKeeper) InflateSupply(ctx sdk.Context, amount sdk.Coins) {
	supplier := keeper.GetSupplier(ctx)
	supplier.CirculatingSupply = supplier.CirculatingSupply.Add(amount)
	keeper.SetSupplier(ctx, supplier)
}

// RequestTokens adds requested tokens to the module's holdings
func (keeper BaseKeeper) RequestTokens(
	ctx sdk.Context, moduleName string, amount sdk.Coins,
) (err error) {
	if !amount.IsValid() {
		return fmt.Errorf("invalid requested amount")
	}

	holder, err := keeper.GetTokenHolder(ctx, moduleName)
	if err != nil {
		return fmt.Errorf("module %s doesn't exist", moduleName)
	}

	supplier := keeper.GetSupplier(ctx)
	supplier.HoldersSupply = supplier.HoldersSupply.Add(amount)

	holder.SetHoldings(holder.GetHoldings().Add(amount))

	keeper.SetTokenHolder(ctx, holder)
	keeper.SetSupplier(ctx, supplier)
	return
}

// RelinquishTokens hands over a portion of the module's holdings
func (keeper BaseKeeper) RelinquishTokens(
	ctx sdk.Context, moduleName string, amount sdk.Coins,
) error {
	if !amount.IsValid() {
		return fmt.Errorf("invalid provided relenquished amount")
	}

	holder, err := keeper.GetTokenHolder(ctx, moduleName)
	if err != nil {
		return fmt.Errorf("module %s doesn't exist", moduleName)
	}

	newHoldings, ok := holder.GetHoldings().SafeSub(amount)
	if !ok {
		return fmt.Errorf("insufficient token holdings")
	}

	supplier := keeper.GetSupplier(ctx)
	newHoldersSupply, ok := supplier.HoldersSupply.SafeSub(amount)
	if !ok {
		panic("total holders supply should be greater than relinquished amount")
	}
	supplier.HoldersSupply = newHoldersSupply

	holder.SetHoldings(newHoldings)

	keeper.SetTokenHolder(ctx, holder)
	keeper.SetSupplier(ctx, supplier)
	return nil
}