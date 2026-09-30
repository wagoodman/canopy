Sharding self-test workload. Test counts and measured time disagree about which package is heaviest: `slow` has one test that sleeps 2s, `wide` has one table test with 24 subtests that returns instantly. There are also nested packages, an external test package, a package with no tests, and gaps in the directory tree.

Used by `.github/workflows/sharding.yaml`. Building with `-tags shardfail` adds a failing test to `tagged/`, so the run fails.
