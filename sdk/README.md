# Kalon JS RPC Client (Lightweight)

A minimal, dependency-free JS client to call the Kalon Network JSON-RPC API from browsers or Node.

- **Default RPC**: `https://explorer.kalon-network.com/rpc`
- **Amounts**: KALON in micro-KALON (1 tKALON = 1,000,000 micro-KALON); token amounts are raw units (no 1e6 scaling).
- **Signing**: The client does **not** manage keys. Sign transactions client-side and send via `sendTransaction`.
- **CORS**: The public endpoint accepts browser requests. For self-hosting, you can use a simple proxy with CORS (e.g., the provided `rpc-proxy.py`).

## Quick Start (Browser ESM)

```html
<script type="module">
  import { createKalonClient } from './sdk/kalon-rpc.js';

  const kalon = createKalonClient(); // defaults to https://explorer.kalon-network.com/rpc

  // Get balances
  const info = await kalon.getAddressInfo('kalon1...youraddress...');
  console.log('Balance micro-KALON:', info.balance);
  console.log('Token balances:', info.tokenBalances);

  // Send token (requires a signed tx if backend expects it; here we call direct RPC method)
  await kalon.sendToken({
    from: 'kalon1sender...',
    to: 'kalon1recipient...',
    token: 'MYTOKEN',
    amount: 12345,        // raw units
    fee: 1_000_000,       // 1 tKALON
  });
</script>
```

## Quick Start (Node ESM)

```js
import { createKalonClient } from './sdk/kalon-rpc.js';
const kalon = createKalonClient('https://explorer.kalon-network.com/rpc');

(async () => {
  const bal = await kalon.getBalance('kalon1...youraddress...');
  console.log(bal);
})();
```

## API

- `new KalonClient(rpcUrl?)` / `createKalonClient(rpcUrl?)`
- `call(method, params)`: low-level JSON-RPC call.
- `getBalance(address)`
- `getAddressInfo(address)` – returns KALON balance, tokenBalances, tx stats.
- `getTokenBalances(address)`
- `checkTokenName(name)`
- `sendTransaction(tx)` – expects a signed transaction object.
- `deployToken({ name, description?, totalSupply, creator, fee? })`
- `sendToken({ from, to, token, amount, fee? })`

## Notes on Signing

This client does not handle private keys. For browser games or dApps:
- Keep keys client-side (never send mnemonics/privkeys to RPC).
- Sign transactions locally, then send via `sendTransaction`.
- Optionally build a small signer module on top of your wallet logic.

## Self-hosted Proxy (optional)

If you need your own domain/CORS handling, you can run a tiny proxy:
- See `explorer/rpc-proxy.py` (serves static files + proxies `/rpc` with CORS headers).

## Security

- Never embed private keys in frontend bundles.
- Always sign locally; RPC should only see signed transactions or unsigned deploy/sendToken calls as designed by the backend.
- Validate user inputs (addresses, amounts, fees) before sending.
