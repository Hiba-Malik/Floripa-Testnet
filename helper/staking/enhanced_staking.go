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
	genesisTotal *big.Int
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

// SetGenesisAllocCache sets the genesis allocation cache
func SetGenesisAllocCache(alloc map[types.Address]*chain.GenesisAccount) {
	GenesisAllocCache = alloc
	// Calculate and cache genesis total
	genesisTotal = calculateGenesisTotal()
}

// calculateGenesisTotal calculates the total premine from genesis allocation
func calculateGenesisTotal() *big.Int {
	if GenesisAllocCache == nil {
		fmt.Println("[GENESIS TOTAL] GenesisAllocCache is nil, returning 0")
		return big.NewInt(0)
	}

	total := big.NewInt(0)
	for addr, acc := range GenesisAllocCache {
		// Skip zero address as it's used for system operations
		if addr == types.ZeroAddress {
			continue
		}
		if acc.Balance != nil {
			total.Add(total, acc.Balance)
		}
	}

	// Convert to AZE for logging
	totalAZE := new(big.Float).Quo(new(big.Float).SetInt(total), big.NewFloat(1e18))
	fmt.Printf("[GENESIS TOTAL] Calculated genesis total: %s AZE (%s wei)\n",
		totalAZE.Text('f', 0), total.String())

	return total
}

// getGenesisTotal returns the cached genesis total
func getGenesisTotal() *big.Int {
	if genesisTotal == nil {
		genesisTotal = calculateGenesisTotal()
	}
	return new(big.Int).Set(genesisTotal) // Return a copy
}

// getCurrentSupplyFromBlockNumber calculates supply using deterministic formula:
// Current Supply = Genesis Total + (Block Number * 1 AZE)
func getCurrentSupplyFromBlockNumber(blockNumber uint64) *big.Int {
	genesisTotal := getGenesisTotal()

	// Calculate block rewards minted so far
	blockRewards := new(big.Int).Mul(
		big.NewInt(int64(blockNumber)),
		big.NewInt(BlockRewardAmount), // 1 AZE per block
	)

	// Total supply = Genesis total + Block rewards
	currentSupply := new(big.Int).Add(genesisTotal, blockRewards)

	// Log for debugging
	genesisAZE := new(big.Float).Quo(new(big.Float).SetInt(genesisTotal), big.NewFloat(1e18))
	blockRewardsAZE := new(big.Float).Quo(new(big.Float).SetInt(blockRewards), big.NewFloat(1e18))
	currentSupplyAZE := new(big.Float).Quo(new(big.Float).SetInt(currentSupply), big.NewFloat(1e18))

	fmt.Printf("[SUPPLY CALC] Block %d: Genesis=%s AZE + BlockRewards=%s AZE = Total=%s AZE\n",
		blockNumber, genesisAZE.Text('f', 0), blockRewardsAZE.Text('f', 0), currentSupplyAZE.Text('f', 0))

	return currentSupply
}

// LogGenesisAllocSum logs the genesis allocation sum for debugging
func LogGenesisAllocSum() *big.Int {
	return getGenesisTotal()
}

func MintBlockReward(
	txn interface {
		AddBalance(types.Address, *big.Int)
		GetBalance(types.Address) *big.Int
	},
	blockNumber uint64,
	ownerAddress types.Address,
) error {
	// Use deterministic supply calculation: Genesis + (Block Number * 1 AZE)
	currentSupply := getCurrentSupplyFromBlockNumber(blockNumber)

	blockReward := big.NewInt(BlockRewardAmount) // 1 AZE

	// Maximum supply: 1 billion AZE
	maxSupply := new(big.Int)
	maxSupply.SetString("1000000000000000000000000000", 10) // 1 billion AZE in wei

	// Log current state
	currentSupplyAZE := new(big.Float).Quo(new(big.Float).SetInt(currentSupply), big.NewFloat(1e18))
	maxSupplyAZE := new(big.Float).Quo(new(big.Float).SetInt(maxSupply), big.NewFloat(1e18))
	fmt.Printf("[SUPPLY CAP] Block %d: New Supply would be = %s AZE, Max Supply = %s AZE\n",
		blockNumber, currentSupplyAZE.Text('f', 0), maxSupplyAZE.Text('f', 0))

	// Check if minting 1 more AZE would exceed the cap
	newSupply := new(big.Int).Add(currentSupply, blockReward)
	if newSupply.Cmp(maxSupply) > 0 {
		fmt.Printf("[SUPPLY CAP] Block %d: Cannot mint full reward - would exceed cap\n", blockNumber)

		// Calculate remaining tokens that can be minted
		remaining := new(big.Int).Sub(maxSupply, currentSupply)
		if remaining.Sign() <= 0 {
			fmt.Printf("[SUPPLY CAP] Block %d: Supply cap reached! No reward minted.\n", blockNumber)
			return nil // Cap already reached, no more minting
		}

		// Mint only the remaining amount to reach cap exactly
		remainingAZE := new(big.Float).Quo(new(big.Float).SetInt(remaining), big.NewFloat(1e18))
		fmt.Printf("[SUPPLY CAP] Block %d: Minting partial reward: %s AZE (remaining to cap)\n",
			blockNumber, remainingAZE.Text('f', 0))

		txn.AddBalance(ownerAddress, remaining)
		return nil
	}

	// We can mint the full 1 AZE reward
	txn.AddBalance(ownerAddress, blockReward)

	finalSupplyAZE := new(big.Float).Quo(new(big.Float).SetInt(newSupply), big.NewFloat(1e18))
	fmt.Printf("[SUPPLY CAP] Block %d: Minted 1 AZE reward. New supply: %s AZE\n",
		blockNumber, finalSupplyAZE.Text('f', 0))

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

// GetCurrentSupplyAtBlock returns the deterministic supply at a given block
func GetCurrentSupplyAtBlock(blockNumber uint64) *big.Int {
	return getCurrentSupplyFromBlockNumber(blockNumber)
}
