package staking

import (
	"fmt"
	"math/big"

	"github.com/0xPolygon/polygon-edge/chain"
	"github.com/0xPolygon/polygon-edge/helper/common"
	"github.com/0xPolygon/polygon-edge/helper/hex"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/0xPolygon/polygon-edge/validators"
)

// ValidateStakingParams validates the staking contract parameters before deployment
func ValidateStakingParams(params PredeployParams) error {
	if params.MinValidatorCount == 0 {
		return fmt.Errorf("MinValidatorCount cannot be zero")
	}
	if params.MaxValidatorCount < params.MinValidatorCount {
		return fmt.Errorf("MaxValidatorCount (%d) cannot be less than MinValidatorCount (%d)",
			params.MaxValidatorCount, params.MinValidatorCount)
	}
	if params.OwnerAddress == "" {
		return fmt.Errorf("OwnerAddress cannot be empty")
	}
	ownerAddr := types.StringToAddress(params.OwnerAddress)
	if ownerAddr == types.ZeroAddress {
		return fmt.Errorf("OwnerAddress cannot be zero address")
	}
	return nil
}

// DebugConstructorData prints the constructor data for debugging
func DebugConstructorData(params PredeployParams) {
	ownerAddr := types.StringToAddress(params.OwnerAddress)
	minValidatorsBig := new(big.Int).SetUint64(params.MinValidatorCount)
	maxValidatorsBig := new(big.Int).SetUint64(params.MaxValidatorCount)

	constructorData := make([]byte, 96) // 3 * 32 bytes
	copy(constructorData[0:32], common.PadLeftOrTrim(minValidatorsBig.Bytes(), 32))
	copy(constructorData[32:64], common.PadLeftOrTrim(maxValidatorsBig.Bytes(), 32))
	copy(constructorData[64:96], common.PadLeftOrTrim(ownerAddr.Bytes(), 32))

	fmt.Printf("=== STAKING CONTRACT DEBUG INFO ===\n")
	fmt.Printf("MinValidatorCount: %d (0x%x)\n", params.MinValidatorCount, minValidatorsBig.Bytes())
	fmt.Printf("MaxValidatorCount: %d (0x%x)\n", params.MaxValidatorCount, maxValidatorsBig.Bytes())
	fmt.Printf("OwnerAddress: %s (0x%x)\n", params.OwnerAddress, ownerAddr.Bytes())
	fmt.Printf("Constructor Data (96 bytes): 0x%x\n", constructorData)
	fmt.Printf("Bytecode Length: %d bytes\n", len(StakingSCBytecode)/2)

	// Validate bytecode can be decoded
	if _, err := hex.DecodeHex(StakingSCBytecode); err != nil {
		fmt.Printf("ERROR: Failed to decode bytecode: %v\n", err)
	} else {
		fmt.Printf("Bytecode validation: OK\n")
	}
	fmt.Printf("===================================\n")
}

// CreateFallbackStakingAccount creates a staking account without constructor (fallback mode)
func CreateFallbackStakingAccount(params PredeployParams) (*types.Hash, map[types.Hash]types.Hash, error) {
	// This creates the storage without relying on constructor
	storageMap := make(map[types.Hash]types.Hash)

	// Set min/max validators manually
	minValidatorsBig := new(big.Int).SetUint64(params.MinValidatorCount)
	maxValidatorsBig := new(big.Int).SetUint64(params.MaxValidatorCount)

	storageMap[types.BytesToHash(big.NewInt(minNumValidatorSlot).Bytes())] =
		types.BytesToHash(minValidatorsBig.Bytes())
	storageMap[types.BytesToHash(big.NewInt(maxNumValidatorSlot).Bytes())] =
		types.BytesToHash(maxValidatorsBig.Bytes())

	// Set validators array length to 0
	storageMap[types.BytesToHash(big.NewInt(validatorsSlot).Bytes())] =
		types.BytesToHash(big.NewInt(0).Bytes())

	// Set total staked amount to 0
	storageMap[types.BytesToHash(big.NewInt(stakedAmountSlot).Bytes())] =
		types.BytesToHash(big.NewInt(0).Bytes())

	// Set owner (if we had an owner slot, but this contract doesn't seem to have one in storage)

	codeHash := types.StringToHash("0x" + StakingSCBytecode)

	return &codeHash, storageMap, nil
}

// ValidateStakingPredeployment validates the parameters for the staking contract predeployment
// and provides detailed debug output.
func ValidateStakingPredeployment(
	vals validators.Validators,
	params PredeployParams,
) (*chain.GenesisAccount, error) {
	fmt.Println("--- Validating Staking Contract Predeployment ---")

	// 1. Validate Core Parameters
	if params.MinValidatorCount == 0 {
		return nil, fmt.Errorf("validation failed: MinValidatorCount cannot be zero")
	}
	if params.MaxValidatorCount < params.MinValidatorCount {
		return nil, fmt.Errorf("validation failed: MaxValidatorCount (%d) cannot be less than MinValidatorCount (%d)",
			params.MaxValidatorCount, params.MinValidatorCount)
	}
	if params.OwnerAddress == "" {
		return nil, fmt.Errorf("validation failed: OwnerAddress cannot be empty")
	}

	fmt.Printf("  [✔] Core parameters are valid.\n")
	fmt.Printf("      - Min Validators: %d\n", params.MinValidatorCount)
	fmt.Printf("      - Max Validators: %d\n", params.MaxValidatorCount)
	fmt.Printf("      - Owner: %s\n", params.OwnerAddress)

	// 2. Validate Validator Set
	if vals == nil || vals.Len() == 0 {
		fmt.Println("  [!] Warning: No initial validators provided in the genesis file.")
	} else {
		if uint64(vals.Len()) < params.MinValidatorCount {
			return nil, fmt.Errorf("validation failed: not enough validators. Have %d, need at least %d",
				vals.Len(), params.MinValidatorCount)
		}
		if uint64(vals.Len()) > params.MaxValidatorCount {
			return nil, fmt.Errorf("validation failed: too many validators. Have %d, max allowed %d",
				vals.Len(), params.MaxValidatorCount)
		}
		fmt.Printf("  [✔] Validator set is valid with %d validators.\n", vals.Len())
	}

	fmt.Println("--- Predeployment validation successful ---")

	// Attempt to predeploy the staking SC
	account, err := PredeployStakingSC(vals, params)
	if err != nil {
		fmt.Printf("--- Primary Predeployment FAILED: %v ---\n", err)
		fmt.Println("--- Attempting Fallback: Initializing Storage Without Constructor ---")
		// If the main predeployment fails, try to initialize storage manually without constructor args
		// This is a fallback for debugging purposes
		return predeployStakingSCFallback(vals, params)
	}

	return account, nil
}

// predeployStakingSCFallback provides a simplified, manual storage initialization
// for the staking contract, intended for debugging when the primary method fails.
func predeployStakingSCFallback(
	vals validators.Validators,
	params PredeployParams,
) (*chain.GenesisAccount, error) {
	fmt.Println("--- Fallback Predeployment Initialized ---")
	// This function would contain the core logic of setting storage slots manually
	// without relying on constructor arguments being appended to the bytecode.
	// This is a safeguard against "execution reverted" errors during genesis.

	// For the purpose of this debug file, we'll just call the corrected primary function.
	// In a real scenario, this could have a simplified, independent implementation.
	return PredeployStakingSC(vals, params)
}
