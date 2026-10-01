### canopy shard join: ❌ failed

| shard | pkgs | est. | actual | tests | result |
|---|---|---|---|---|---|
| 1/3 | 2 | 0s | 2s | 10 passed | ✅ |
| 2/3 | 2 | 0s | 2s | 11 passed / 1 skipped | ✅ |
| 3/3 | 2 | 0s | 2s | 12 passed / 2 skipped | ✅ |

- ❌ **verified**: [run] shard 2 only: test-flag -run=TestDoesNotExist (a flag or CANOPY_TEST_* env var is set on some jobs only)
- ✅ **coverage**: 75.0%
