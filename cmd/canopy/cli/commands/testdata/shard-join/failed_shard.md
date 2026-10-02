### canopy shard join: ❌ failed

| shard | pkgs | est. | actual | tests | result |
|---|---|---|---|---|---|
| 1/3 | 2 | 0s | 2s | 10 passed | ✅ |
| 2/3 | 2 | 0s | 2s | 11 passed / 1 failed / 1 skipped | ❌ |
| 3/3 | 2 | 0s | 2s | 12 passed / 2 skipped | ✅ |

- ❌ **tests**: `TestReconcile` in `m/d` (shard 2/3)
- ✅ **verified**: 6 of 6 packages ran exactly once
- ✅ **coverage**: 75.0%

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
