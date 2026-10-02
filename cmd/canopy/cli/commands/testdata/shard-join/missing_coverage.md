### canopy shard join: ❌ failed

| shard | pkgs | est. | actual | tests | result |
|---|---|---|---|---|---|
| 1/3 | 2 | 0s | 2s | 10 passed | ✅ |
| 2/3 | - | - | - | - | ❌ missing |
| 3/3 | 2 | 0s | 2s | 12 passed / 2 skipped | ✅ |

- ❌ **verified**
  - missing receipt for shard 2/3 (job failed before canopy finished, or artifact not uploaded)
- ❌ **coverage**: coverage below threshold: 78.30% < 80.00%
- ⚠️ canopy version differs between shards
