### canopy shard join: ❌ failed

| shard | pkgs | est. | actual | tests | result |
|---|---|---|---|---|---|
| 1/3 | 2 | 0s | 2s | 10 passed | ✅ |
| 2/3 | 2 | 0s | 2s | 11 passed / 1 skipped | ✅ |
| 3/3 | 2 | - | 2s | 12 passed / 2 skipped | ✅ |

- ❌ **verified**
  - [weights] shards 1,2: metrics sha256:451171ec (6 measured, 0 estimated) / shard 3: static (no metrics file) (a cache race, or a partial rerun after main saved new metrics; rerun all jobs)
- ✅ **coverage**: 75.0%
