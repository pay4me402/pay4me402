# Concept:
- It's a proxy configured to use multiple wallets from different chains, agents can connect to it and send requests to 402 protected resources through it.



# Name:

### naming criteria
- indicate 402.
- indicate it's a proxy.
- indicate parental control.
- indicate it's a wallet manager.

### possible names:
- walletproxy.
- 402proxy.
- pay4me proxy.
- agent wallet daddy.




# configurations
### adding wallets:
- Give it a uniqe name/alias.
- select chain between Algorand, and Solona.
- Provide it's private key / secret.

### adding proxy users:
* username
* password

### adding ACL:
* username.
* allowed wallets.
* Optional limit.


### Save stats
* username
* wallet
* amount spent

### UI
- create wallets.
- create proxy users.
- create ACL.
- create budgets.
- view stats.


# Arch
- DB (SQLite3):
    - stores configurations and stats.
- HTTP proxy that handles 402 payments:
    - Handles SSL.
    - Forwards requests to the target server.
    - if 402 received it handles the payment as follows:
        - If yes, it checks ACLs, and budgets, and if a wallet with that supports one of the supported chains exists.        
        - handles payment.
        - returns the response to the end user.
- UI (Web UI for configurations).


# Milestones

### Milestone 1:
- No config, No auth, basic http proxy, single client, configured with Algorand testnet, try with https://x402.goplausible.xyz/examples/weather.

### Milestone 2:
- Configurations: Support Solana Network, test with https://pro-api.coingecko.com/api/v3/x402/onchain/simple/networks/eth/token_price/0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2.

- Decide on configuration structure and whether to use a basic DB or a json file.

- Implement the wallets only and the logic that decides on the network and matches it with one of the available wallets.

### Milestone 3:
- ACLs and budget logic.

### Milestone 4:
- Configurations UI => Web or TUI, or both.


### Milestone 5:
- Natural language ACLs for example you can crate a rule to allow or block X services eg: financial services or weather services.



# Future possibilities:
- On top of our service we can build a service that allows users to pay with legacy payment methods create a corresponding Wallet for each user.
