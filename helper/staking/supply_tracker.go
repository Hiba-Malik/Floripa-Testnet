package staking

import (
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/0xPolygon/polygon-edge/types"
)

const (
	// Supply tracking constants
	MaxSupplyKey   = "max_supply"
	TotalSupplyKey = "total_supply"
	SupplyAuditKey = "supply_audit"

	// Maximum supply: 1 billion AZE
	MaxSupplyAmount = "1000000000000000000000000000" // 1 billion AZE in wei
)

var (
	ErrSupplyCapExceeded  = errors.New("supply cap exceeded")
	ErrUnauthorizedMint   = errors.New("unauthorized mint operation")
	ErrInvalidAmount      = errors.New("invalid amount")
	ErrInsufficientSupply = errors.New("insufficient supply to burn")
)

// SupplyAuditLog represents an immutable record of supply changes
type SupplyAuditLog struct {
	BlockNumber uint64   `json:"blockNumber"`
	Amount      *big.Int `json:"amount"`
	Type        string   `json:"type"` // "mint" or "burn"
	Timestamp   uint64   `json:"timestamp"`
	Caller      string   `json:"caller"`
}

// SupplyTracker manages secure supply tracking
type SupplyTracker struct {
	initialSupply *big.Int
	auditLog      []SupplyAuditLog
	mutex         sync.RWMutex
}

// NewSupplyTracker creates a new supply tracker
func NewSupplyTracker(initialSupply *big.Int) *SupplyTracker {
	return &SupplyTracker{
		initialSupply: initialSupply,
		auditLog:      make([]SupplyAuditLog, 0),
	}
}

// GetTotalSupply calculates total supply from initial supply + all changes
func (st *SupplyTracker) GetTotalSupply() *big.Int {
	st.mutex.RLock()
	defer st.mutex.RUnlock()

	total := new(big.Int).Set(st.initialSupply)
	for _, change := range st.auditLog {
		if change.Type == "mint" {
			total.Add(total, change.Amount)
		} else if change.Type == "burn" {
			total.Sub(total, change.Amount)
		}
	}
	return total
}

// Mint securely mints new tokens (only callable from consensus engine)
func (st *SupplyTracker) Mint(amount *big.Int, blockNumber uint64, caller string) error {
	if amount == nil || amount.Cmp(big.NewInt(0)) <= 0 {
		return ErrInvalidAmount
	}

	st.mutex.Lock()
	defer st.mutex.Unlock()

	// Validate caller is consensus engine
	if !isConsensusEngine(caller) {
		return ErrUnauthorizedMint
	}

	// The cap check is now handled in MintBlockReward, so we only log here.
	// This prevents a double-check that was causing the partial reward to be rejected.
	currentSupply := st.getCurrentSupply()
	maxSupply := getMaxSupply()
	newSupply := new(big.Int).Add(currentSupply, amount)

	if newSupply.Cmp(maxSupply) > 0 {
		// We still check here to be absolutely safe, but the primary logic
		// in MintBlockReward should prevent this from being reached with an invalid amount.
		// If it is reached, we return the error to halt the process.
		return ErrSupplyCapExceeded
	}

	// Log the mint operation
	st.auditLog = append(st.auditLog, SupplyAuditLog{
		BlockNumber: blockNumber,
		Amount:      amount,
		Type:        "mint",
		Timestamp:   uint64(time.Now().Unix()),
		Caller:      caller,
	})

	return nil
}

// Burn securely burns tokens (only callable from consensus engine)
func (st *SupplyTracker) Burn(amount *big.Int, blockNumber uint64, caller string) error {
	if amount == nil || amount.Cmp(big.NewInt(0)) <= 0 {
		return ErrInvalidAmount
	}

	st.mutex.Lock()
	defer st.mutex.Unlock()

	// Validate caller is consensus engine
	if !isConsensusEngine(caller) {
		return ErrUnauthorizedMint
	}

	// Check sufficient supply
	currentSupply := st.getCurrentSupply()
	if currentSupply.Cmp(amount) < 0 {
		return ErrInsufficientSupply
	}

	// Log the burn operation
	st.auditLog = append(st.auditLog, SupplyAuditLog{
		BlockNumber: blockNumber,
		Amount:      amount,
		Type:        "burn",
		Timestamp:   uint64(time.Now().Unix()),
		Caller:      caller,
	})

	return nil
}

// GetAuditLog returns the immutable audit trail
func (st *SupplyTracker) GetAuditLog() []SupplyAuditLog {
	st.mutex.RLock()
	defer st.mutex.RUnlock()

	// Return a copy to prevent external modification
	logCopy := make([]SupplyAuditLog, len(st.auditLog))
	copy(logCopy, st.auditLog)
	return logCopy
}

// getCurrentSupply calculates current supply (internal use)
func (st *SupplyTracker) getCurrentSupply() *big.Int {
	total := new(big.Int).Set(st.initialSupply)
	for _, change := range st.auditLog {
		if change.Type == "mint" {
			total.Add(total, change.Amount)
		} else if change.Type == "burn" {
			total.Sub(total, change.Amount)
		}
	}
	return total
}

// isConsensusEngine validates if the caller is the consensus engine
func isConsensusEngine(caller string) bool {
	// Accept both the system address and consensus_engine identifier
	consensusEngineAddress := "0x0000000000000000000000000000000000000000" // System address
	consensusEngineIdentifier := "consensus_engine"                        // System identifier
	return caller == consensusEngineAddress || caller == consensusEngineIdentifier
}

// getMaxSupply returns the maximum supply limit
func getMaxSupply() *big.Int {
	maxSupply, _ := new(big.Int).SetString(MaxSupplyAmount, 10)
	return maxSupply
}

// System-level supply tracking functions for use in consensus engine
type SystemSupplyTracker struct {
	tracker *SupplyTracker
}

// NewSystemSupplyTracker creates a system-level supply tracker
func NewSystemSupplyTracker(initialSupply *big.Int) *SystemSupplyTracker {
	return &SystemSupplyTracker{
		tracker: NewSupplyTracker(initialSupply),
	}
}

// MintBlockReward securely mints block rewards by calling the internal mint function.
func (sst *SystemSupplyTracker) MintBlockReward(amount *big.Int, blockNumber uint64) error {
	return sst.tracker.Mint(amount, blockNumber, "consensus_engine")
}

// MintRewardWithCap performs a secure, atomic check-and-mint operation for block rewards.
// It ensures the total supply does not exceed the maximum cap.
func (sst *SystemSupplyTracker) MintRewardWithCap(txn interface {
	AddBalance(types.Address, *big.Int)
}, blockNumber uint64, ownerAddress types.Address) error {
	sst.tracker.mutex.Lock()
	defer sst.tracker.mutex.Unlock()

	currentSupply := sst.tracker.getCurrentSupply()
	maxSupply := getMaxSupply()

	// Convert to AZE for logging
	currentSupplyAZE := new(big.Float).Quo(new(big.Float).SetInt(currentSupply), big.NewFloat(1e18))
	maxSupplyAZE := new(big.Float).Quo(new(big.Float).SetInt(maxSupply), big.NewFloat(1e18))

	// Log current state
	fmt.Printf("[SUPPLY CAP] Block %d: Current Supply = %s AZE, Max Supply = %s AZE\n",
		blockNumber, currentSupplyAZE.Text('f', 0), maxSupplyAZE.Text('f', 0))

	// If we've already reached or exceeded the max supply, do nothing.
	if currentSupply.Cmp(maxSupply) >= 0 {
		fmt.Printf("[SUPPLY CAP] Block %d: Supply cap reached! No reward minted.\n", blockNumber)
		return nil
	}

	blockReward := big.NewInt(BlockRewardAmount)
	originalRewardAZE := new(big.Float).Quo(new(big.Float).SetInt(blockReward), big.NewFloat(1e18))

	// Check if adding the full reward would exceed the max supply.
	newSupply := new(big.Int).Add(currentSupply, blockReward)
	if newSupply.Cmp(maxSupply) > 0 {
		// Only calculate the remaining amount to mint to hit the cap exactly.
		blockReward = new(big.Int).Sub(maxSupply, currentSupply)
		if blockReward.Cmp(big.NewInt(0)) <= 0 {
			fmt.Printf("[SUPPLY CAP] Block %d: No remaining tokens to mint.\n", blockNumber)
			return nil // No remainder to mint.
		}

		partialRewardAZE := new(big.Float).Quo(new(big.Float).SetInt(blockReward), big.NewFloat(1e18))
		fmt.Printf("[SUPPLY CAP] Block %d: Partial reward calculated. Original: %s AZE, Partial: %s AZE\n",
			blockNumber, originalRewardAZE.Text('f', 0), partialRewardAZE.Text('f', 0))
	} else {
		fmt.Printf("[SUPPLY CAP] Block %d: Full reward of %s AZE will be minted.\n",
			blockNumber, originalRewardAZE.Text('f', 0))
	}

	// Now, perform the mint operation within the lock.
	sst.tracker.auditLog = append(sst.tracker.auditLog, SupplyAuditLog{
		BlockNumber: blockNumber,
		Amount:      blockReward,
		Type:        "mint",
		Timestamp:   uint64(time.Now().Unix()),
		Caller:      "consensus_engine",
	})

	// Add the balance to the owner address.
	txn.AddBalance(ownerAddress, blockReward)

	// Log final state
	finalSupply := new(big.Int).Add(currentSupply, blockReward)
	finalSupplyAZE := new(big.Float).Quo(new(big.Float).SetInt(finalSupply), big.NewFloat(1e18))
	rewardAZE := new(big.Float).Quo(new(big.Float).SetInt(blockReward), big.NewFloat(1e18))

	fmt.Printf("[SUPPLY CAP] Block %d: Reward minted! Amount: %s AZE, New Supply: %s AZE\n",
		blockNumber, rewardAZE.Text('f', 0), finalSupplyAZE.Text('f', 0))

	return nil
}

// GetCurrentSupply gets the current total supply
func (sst *SystemSupplyTracker) GetCurrentSupply() *big.Int {
	return sst.tracker.GetTotalSupply()
}

// GetAuditLog gets the supply audit log
func (sst *SystemSupplyTracker) GetAuditLog() []SupplyAuditLog {
	return sst.tracker.GetAuditLog()
}
