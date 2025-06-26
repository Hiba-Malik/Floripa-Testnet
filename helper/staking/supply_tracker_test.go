package staking

import (
	"math/big"
	"testing"
)

func TestSupplyTracker(t *testing.T) {
	// Test 1: Initialize supply tracker
	initialSupply := big.NewInt(1000000) // 1M AZE initial supply
	tracker := NewSupplyTracker(initialSupply)

	if tracker.GetTotalSupply().Cmp(initialSupply) != 0 {
		t.Errorf("Expected initial supply %s, got %s", initialSupply.String(), tracker.GetTotalSupply().String())
	}

	// Test 2: Mint block rewards
	blockReward := big.NewInt(1000000000000000000) // 1 AZE
	err := tracker.Mint(blockReward, 1, "consensus_engine")
	if err != nil {
		t.Errorf("Failed to mint: %v", err)
	}

	expectedSupply := new(big.Int).Add(initialSupply, blockReward)
	if tracker.GetTotalSupply().Cmp(expectedSupply) != 0 {
		t.Errorf("Expected supply after mint %s, got %s", expectedSupply.String(), tracker.GetTotalSupply().String())
	}

	// Test 3: Check audit log
	auditLog := tracker.GetAuditLog()
	if len(auditLog) != 1 {
		t.Errorf("Expected 1 audit log entry, got %d", len(auditLog))
	}

	if auditLog[0].Type != "mint" {
		t.Errorf("Expected audit log type 'mint', got '%s'", auditLog[0].Type)
	}

	if auditLog[0].Amount.Cmp(blockReward) != 0 {
		t.Errorf("Expected audit log amount %s, got %s", blockReward.String(), auditLog[0].Amount.String())
	}

	// Test 4: Burn tokens
	burnAmount := big.NewInt(500000000000000000) // 0.5 AZE
	err = tracker.Burn(burnAmount, 2, "consensus_engine")
	if err != nil {
		t.Errorf("Failed to burn: %v", err)
	}

	expectedSupplyAfterBurn := new(big.Int).Sub(expectedSupply, burnAmount)
	if tracker.GetTotalSupply().Cmp(expectedSupplyAfterBurn) != 0 {
		t.Errorf("Expected supply after burn %s, got %s", expectedSupplyAfterBurn.String(), tracker.GetTotalSupply().String())
	}

	// Test 5: Check final audit log
	finalAuditLog := tracker.GetAuditLog()
	if len(finalAuditLog) != 2 {
		t.Errorf("Expected 2 audit log entries, got %d", len(finalAuditLog))
	}

	if finalAuditLog[1].Type != "burn" {
		t.Errorf("Expected second audit log type 'burn', got '%s'", finalAuditLog[1].Type)
	}
}

func TestSupplyTrackerSecurity(t *testing.T) {
	tracker := NewSupplyTracker(big.NewInt(0))

	// Test: Unauthorized mint should fail
	err := tracker.Mint(big.NewInt(1000000), 1, "unauthorized_caller")
	if err == nil {
		t.Error("Expected unauthorized mint to fail")
	}

	if err != ErrUnauthorizedMint {
		t.Errorf("Expected ErrUnauthorizedMint, got %v", err)
	}

	// Test: Invalid amount should fail
	err = tracker.Mint(big.NewInt(0), 1, "consensus_engine")
	if err == nil {
		t.Error("Expected invalid amount to fail")
	}

	if err != ErrInvalidAmount {
		t.Errorf("Expected ErrInvalidAmount, got %v", err)
	}

	// Test: Negative amount should fail
	err = tracker.Mint(big.NewInt(-1), 1, "consensus_engine")
	if err == nil {
		t.Error("Expected negative amount to fail")
	}
}

func TestSupplyTrackerMaxSupply(t *testing.T) {
	// Start with max supply - 1 AZE
	maxSupplyBig, _ := new(big.Int).SetString(MaxSupplyAmount, 10)
	initialSupply := new(big.Int).Sub(maxSupplyBig, big.NewInt(1000000000000000000)) // Max - 1 AZE

	tracker := NewSupplyTracker(initialSupply)

	// Try to mint 2 AZE (should only mint 1 AZE to reach max)
	blockReward := big.NewInt(2000000000000000000) // 2 AZE
	err := tracker.Mint(blockReward, 1, "consensus_engine")
	if err != nil {
		t.Errorf("Failed to mint: %v", err)
	}

	// Should have reached max supply
	if tracker.GetTotalSupply().Cmp(maxSupplyBig) != 0 {
		t.Errorf("Expected max supply %s, got %s", maxSupplyBig.String(), tracker.GetTotalSupply().String())
	}

	// Try to mint more (should fail)
	err = tracker.Mint(big.NewInt(1000000000000000000), 2, "consensus_engine")
	if err == nil {
		t.Error("Expected minting beyond max supply to fail")
	}

	if err != ErrSupplyCapExceeded {
		t.Errorf("Expected ErrSupplyCapExceeded, got %v", err)
	}
}
