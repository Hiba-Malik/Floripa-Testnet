# Genesis and Validator Setup in Azore Mainnet

## 1. Initial Mint to Owner

- At genesis, the owner address receives an initial allocation of **100,000 AZE**.
- This is specified in the `genesis.json` file under the `alloc` section, mapping the owner address to the initial balance.

## 2. Two Validators in Genesis

- The genesis block includes two validator addresses, each with their own initial stake (as specified in `genesis.json`).
- These validators are responsible for producing and validating blocks from the start of the chain.
- Their addresses and stakes are defined in the `validators` and `alloc` sections of the genesis configuration.

## 3. Adding New Validators

- **Staking Requirement:**
  - Any new participant who wants to become a validator must first **stake the required amount of AZE tokens** to the staking contract.
- **Registration Process:**
  1. **Stake Tokens:** Send a transaction to the staking contract to lock up the required amount of AZE.
  2. **Register IBS Public Key:** Call the `registerIBS` function on the staking contract, providing your IBS (Identity-Based Signature) public key.
  3. **Activation:** Once both steps are complete and the transaction is confirmed, the new validator will be eligible to participate in block production and validation.

- **Note:**
  - Only addresses that have staked and registered their IBS public key will be recognized as active validators by the consensus engine.
  - The process ensures that only properly staked and registered participants can validate and produce blocks, maintaining the security and integrity of the network. 