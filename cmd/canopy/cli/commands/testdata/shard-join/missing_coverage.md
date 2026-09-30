### canopy shard join: ❌ failed

| shard | pkgs | est. | actual | tests | result |
|---|---|---|---|---|---|
| 1/3 | 2 | 0s | 2s | 10 passed | ✅ |
| 2/3 | - | - | - | missing | ❌ |
| 3/3 | 2 | 0s | 2s | 12 passed / 2 skipped | ✅ |

- ❌ **verified**: missing receipt for shard 2/3 (job failed before canopy finished, or artifact not uploaded)
- ❌ **coverage**: coverage below threshold: 78.30% < 80.00%
- ⚠️ canopy version differs between shards

<details><summary>shard count suggestion</summary>

| shards | est. wall | runner time |
|---|---|---|
| 1 | 1s | 2s |
| **2** | **1s** | **3s** |
| 3 | 1s | 4s |
| 4 | 1s | 5s |
| 5 | 1s | 6s |
| 6 | 1s | 7s |

</details>
