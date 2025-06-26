package staking

import (
	"fmt"
	"math/big"

	"github.com/0xPolygon/polygon-edge/chain"
	"github.com/0xPolygon/polygon-edge/types"
)

const (
	// Block reward amount (1 AZE)
	BlockRewardAmount = 1000000000000000000 // 1 * 10^18 wei
)

var (
	// Global supply tracker instance
	globalSupplyTracker *SystemSupplyTracker
	// Cache for genesis premine to avoid recalculating
	genesisPremine *big.Int
	// Global cache for genesis Alloc
	GenesisAllocCache map[types.Address]*chain.GenesisAccount
)

// StateTransition interface to abstract the state transition operations
type StateTransition interface {
	AddBalance(addr types.Address, amount *big.Int)
	GetBalance(addr types.Address) *big.Int
	Call2(caller types.Address, to types.Address, input []byte, value *big.Int, gas uint64) ExecutionResult
	AccountExists(addr types.Address) bool
}

// ExecutionResult interface to abstract execution results
type ExecutionResult interface {
	Failed() bool
	GetErr() error
}

// InitializeSupplyTracker initializes the global supply tracker
func InitializeSupplyTracker(initialSupply *big.Int) {
	globalSupplyTracker = NewSystemSupplyTracker(initialSupply)
}

// UpdateSupplyTrackerWithTotalSupply updates the supply tracker with the actual total supply
func UpdateSupplyTrackerWithTotalSupply(totalSupply *big.Int) {
	if globalSupplyTracker == nil {
		globalSupplyTracker = NewSystemSupplyTracker(totalSupply)
		return
	}

	// Update the initial supply in the existing tracker
	globalSupplyTracker.tracker.initialSupply = totalSupply
}

// GetGlobalSupplyTracker returns the global supply tracker instance
func GetGlobalSupplyTracker() *SystemSupplyTracker {
	if globalSupplyTracker == nil {
		// Initialize with zero if not already initialized
		globalSupplyTracker = NewSystemSupplyTracker(big.NewInt(0))
	}
	return globalSupplyTracker
}

// CalculateGenesisPremineFromConfig calculates the total premine from genesis allocation
func CalculateGenesisPremineFromConfig(genesisAlloc map[types.Address]*chain.GenesisAccount) *big.Int {
	if genesisPremine != nil {
		return genesisPremine
	}

	totalPremine := big.NewInt(0)

	for addr, alloc := range genesisAlloc {
		// Skip zero address as it's used for minting/burning
		if addr == types.ZeroAddress {
			continue
		}
		if alloc.Balance != nil {
			totalPremine.Add(totalPremine, alloc.Balance)
		}
	}

	// Cache the result
	genesisPremine = totalPremine

	// Log the calculated premine for debugging
	premineAZE := new(big.Float).Quo(new(big.Float).SetInt(totalPremine), big.NewFloat(1e18))
	fmt.Printf("[GENESIS PREMINE] Calculated total premine: %s AZE\n", premineAZE.Text('f', 0))

	return totalPremine
}

// InitializeGenesisPremineFromConfig initializes the genesis premine from the chain config
func InitializeGenesisPremineFromConfig(genesisAlloc map[types.Address]*chain.GenesisAccount) {
	CalculateGenesisPremineFromConfig(genesisAlloc)
}

// ClearGenesisCache clears the cached genesis premine and alloc cache
// Call this when you want to force reload of genesis data
func ClearGenesisCache() {
	genesisPremine = nil
	GenesisAllocCache = nil
	fmt.Println("[GENESIS CACHE] Cleared genesis premine and alloc cache")
}

// GetGenesisPremineOrDefault returns the cached genesis premine or the sum from GenesisAllocCache
func GetGenesisPremineOrDefault() *big.Int {
	if genesisPremine != nil {
		return genesisPremine
	}

	// If GenesisAllocCache is set, sum all balances (except zero address)
	if GenesisAllocCache != nil {
		total := big.NewInt(0)
		for addr, acc := range GenesisAllocCache {
			if addr == types.ZeroAddress {
				continue
			}
			if acc.Balance != nil {
				total.Add(total, acc.Balance)
			}
		}
		return total
	}

	// Fallback to zero if not initialized
	return big.NewInt(0)
}

// LogGenesisAllocSum logs the sum of all genesis premine balances
func LogGenesisAllocSum() *big.Int {
	if GenesisAllocCache == nil {
		fmt.Println("[GENESIS ALLOC] GenesisAllocCache is nil")
		return big.NewInt(0)
	}
	total := big.NewInt(0)
	for addr, acc := range GenesisAllocCache {
		if addr == types.ZeroAddress {
			continue
		}
		if acc.Balance != nil {
			total.Add(total, acc.Balance)
		}
	}
	fmt.Printf("[GENESIS ALLOC] Total premine sum: %s\n", total.String())
	return total
}

func getActualTotalSupply(txn interface {
	GetBalance(types.Address) *big.Int
}) *big.Int {
	total := big.NewInt(0)
	if GenesisAllocCache != nil {
		for addr := range GenesisAllocCache {
			if addr == types.ZeroAddress {
				continue
			}
			bal := txn.GetBalance(addr)
			if bal != nil {
				total.Add(total, bal)
			}
		}
	}
	return total
}

func MintBlockReward(
	txn interface {
		AddBalance(types.Address, *big.Int)
		GetBalance(types.Address) *big.Int
	},
	blockNumber uint64,
	ownerAddress types.Address,
) error {
	// Get the actual current supply from state
	currentSupply := getActualTotalSupply(txn)

	blockReward := big.NewInt(BlockRewardAmount)

	// Maximum supply: 1 billion AZE
	maxSupply := new(big.Int)
	maxSupply.SetString("1000000000000000000000000000", 10) // 1 billion AZE in wei

	// Log for debugging
	currentSupplyAZE := new(big.Float).Quo(new(big.Float).SetInt(currentSupply), big.NewFloat(1e18))
	maxSupplyAZE := new(big.Float).Quo(new(big.Float).SetInt(maxSupply), big.NewFloat(1e18))
	fmt.Printf("[SUPPLY CAP] Block %d: Current Supply = %s AZE, Max Supply = %s AZE\n", blockNumber, currentSupplyAZE.Text('f', 0), maxSupplyAZE.Text('f', 0))

	if currentSupply.Cmp(maxSupply) >= 0 {
		fmt.Printf("[SUPPLY CAP] Block %d: Supply cap reached! No reward minted.\n", blockNumber)
		return ErrSupplyCapExceeded
	}

	remaining := new(big.Int).Sub(maxSupply, currentSupply)
	mintAmount := blockReward
	if remaining.Cmp(blockReward) < 0 {
		mintAmount = remaining
	}

	if mintAmount.Sign() > 0 {
		txn.AddBalance(ownerAddress, mintAmount)
		fmt.Printf("[SUPPLY CAP] Block %d: Minted %s wei as reward (up to cap)\n", blockNumber, mintAmount.String())
	} else {
		fmt.Printf("[SUPPLY CAP] Block %d: No reward minted (already at cap)\n", blockNumber)
	}

	return nil
}

// DistributeTxFeesToValidator distributes transaction fees: 50% to owner, 50% to block producer
func DistributeTxFeesToValidator(
	txn interface{ AddBalance(types.Address, *big.Int) },
	totalFees *big.Int,
	ownerAddress types.Address,
	blockProducerAddress types.Address,
) error {
	if totalFees == nil || totalFees.Cmp(big.NewInt(0)) == 0 {
		return nil
	}

	// Split fees: 50% to owner, 50% to block producer (validator)
	ownerFee := new(big.Int).Div(totalFees, big.NewInt(2))
	validatorFee := new(big.Int).Sub(totalFees, ownerFee)

	// Transfer fees
	txn.AddBalance(ownerAddress, ownerFee)
	txn.AddBalance(blockProducerAddress, validatorFee)

	return nil
}

// CheckStakingContractDeployed checks if the staking contract is deployed
func CheckStakingContractDeployed(
	transition interface{ AccountExists(types.Address) bool },
) bool {
	stakingAddress := types.StringToAddress("0x0000000000000000000000000000000000001001")
	return transition.AccountExists(stakingAddress)
}

// GetCurrentSupply gets the current total supply from the secure supply tracker
func GetCurrentSupply() *big.Int {
	supplyTracker := GetGlobalSupplyTracker()
	return supplyTracker.GetCurrentSupply()
}

// GetSupplyAuditLog gets the supply audit log for transparency
func GetSupplyAuditLog() []SupplyAuditLog {
	supplyTracker := GetGlobalSupplyTracker()
	return supplyTracker.GetAuditLog()
}
