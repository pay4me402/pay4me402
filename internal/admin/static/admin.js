const rpcEndpointsByChain = {
  algorand: [
    ["", "Default: TestNet AlgoNode (https://testnet-api.algonode.cloud)"],
    ["https://testnet-api.algonode.cloud", "TestNet AlgoNode"],
    ["https://mainnet-api.algonode.cloud", "MainNet AlgoNode"],
    ["https://betanet-api.algonode.cloud", "BetaNet AlgoNode"],
    ["custom", "Custom endpoint"]
  ],
  solana: [
    ["", "Default: MainNet Beta (https://api.mainnet-beta.solana.com)"],
    ["https://api.mainnet-beta.solana.com", "MainNet Beta"],
    ["https://api.devnet.solana.com", "DevNet"],
    ["https://api.testnet.solana.com", "TestNet"],
    ["custom", "Custom endpoint"]
  ]
};

const walletChain = document.getElementById("wallet-chain");
const walletRpcEndpoint = document.getElementById("wallet-rpc-endpoint");
const customRpcEndpoint = document.getElementById("custom-rpc-endpoint");

function updateRpcEndpointOptions() {
  if (!walletChain || !walletRpcEndpoint || !customRpcEndpoint) return;
  const options = rpcEndpointsByChain[walletChain.value] || [];
  walletRpcEndpoint.replaceChildren(...options.map(([value, label]) => {
    const option = document.createElement("option");
    option.value = value;
    option.textContent = label;
    return option;
  }));
  updateCustomRpcEndpoint();
}

function updateCustomRpcEndpoint() {
  if (!walletRpcEndpoint || !customRpcEndpoint) return;
  const custom = walletRpcEndpoint.value === "custom";
  customRpcEndpoint.disabled = !custom;
  customRpcEndpoint.required = custom;
  if (!custom) customRpcEndpoint.value = "";
}

walletChain?.addEventListener("change", updateRpcEndpointOptions);
walletRpcEndpoint?.addEventListener("change", updateCustomRpcEndpoint);
updateRpcEndpointOptions();
