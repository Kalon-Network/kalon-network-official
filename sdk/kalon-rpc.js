/**
 * Kalon Network - Lightweight RPC Client (Browser/Node, zero deps)
 * Default endpoint: https://explorer.kalon-network.com/rpc
 *
 * Notes:
 * - KALON amounts are in micro-KALON (1 tKALON = 1,000,000 micro-KALON).
 * - Token amounts are raw units (no implicit 1e6 scaling).
 * - This client does NOT manage keys or signing. Sign transactions client-side.
 */

const DEFAULT_RPC = "https://explorer.kalon-network.com/rpc";

function normalizeRpcUrl(url) {
    if (!url) return DEFAULT_RPC;
    if (url.endsWith("/rpc")) return url;
    if (url.endsWith("/")) return `${url}rpc`;
    return `${url}/rpc`;
}

export class KalonClient {
    constructor(rpcUrl = DEFAULT_RPC) {
        this.rpcUrl = normalizeRpcUrl(rpcUrl);
    }

    async call(method, params = {}) {
        const payload = {
            jsonrpc: "2.0",
            method,
            params,
            id: Date.now(),
        };
        const resp = await fetch(this.rpcUrl, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(payload),
        });
        const data = await resp.json();
        if (data.error) {
            const msg = data.error.message || "RPC Error";
            throw new Error(msg);
        }
        return data.result;
    }

    // ---- Read operations ----
    getBalance(address) {
        return this.call("getBalance", { address });
    }

    getAddressInfo(address) {
        return this.call("getAddressInfo", { address });
    }

    getTokenBalances(address) {
        return this.call("getTokenBalances", { address });
    }

    checkTokenName(name) {
        return this.call("checkTokenName", { name });
    }

    // ---- Write operations (assumes caller signs tx as needed) ----
    // Generic transaction sender (signed tx expected)
    sendTransaction(tx) {
        return this.call("sendTransaction", { transaction: tx });
    }

    // Deploy a token (fee: 10 KALON + base fee; amounts in raw units)
    deployToken({ name, description = "", totalSupply, creator, fee = 1_000_000 }) {
        return this.call("deployToken", {
            name,
            description,
            totalSupply,
            creator,
            fee,
        });
    }

    // Send an existing token (fee default 1 tKALON)
    sendToken({ from, to, token, amount, fee = 1_000_000 }) {
        return this.call("sendToken", {
            from,
            to,
            tokenName: token,
            amount,
            fee,
        });
    }
}

// Simple factory for quick one-liners
export function createKalonClient(rpcUrl) {
    return new KalonClient(rpcUrl);
}
