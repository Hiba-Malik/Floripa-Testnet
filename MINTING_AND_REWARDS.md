# Minting and Rewards Logic in Azore Mainnet

## 1. Minting 1 AZE per Block Until 1 Billion Cap

- **Goal:** Ensure that exactly 1 AZE is minted to the owner address for each block, until the total supply of AZE reaches 1,000,000,000 (1 billion) AZE.
- **How it was implemented:**
  - The code calculates the **actual total supply** by summing all balances in the state (excluding the zero address) before minting each block reward.
  - Before minting, it checks if minting 1 AZE would exceed the cap. If so, it only mints the remaining amount to hit the cap exactly. If the cap is reached, no more rewards are minted.
  - This logic is implemented in `helper/staking/enhanced_staking.go` in the `MintBlockReward` function, which uses a helper to sum all balances and enforce the cap strictly.
  - The owner address is specified in the consensus hooks and receives the block reward.

## 2. Transaction Fee Distribution (50/50)

- **Goal:** Split transaction fees so that 50% go to the staker (block producer) and 50% go to the owner.
- **How it was implemented:**
  - The function `DistributeTxFeesToValidator` in `helper/staking/enhanced_staking.go` is called during block processing.
  - It takes the total transaction fees for the block, splits them in half, and distributes 50% to the owner and 50% to the block producer (staker).
  - This ensures fair reward sharing between the protocol owner and active validators.

## 3. Key Points

- The supply cap is enforced **before** minting, so the cap is never exceeded.
- The actual supply is always calculated from the real state, not theoretical rewards.
- All logic is implemented in the staking helper and consensus hooks, ensuring robust enforcement. 