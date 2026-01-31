package xflagset

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestCollectNamedFlagSets(t *testing.T) {
	tests := []struct {
		name      string
		input     any
		wantOrder []string
	}{
		{
			name: "simple struct with one NamedFlagSet",
			input: struct {
				NamedFlagSet *Named
			}{
				NamedFlagSet: func() *Named {
					n := NewNamed()
					n.FlagSet("group1")
					return n
				}(),
			},
			wantOrder: []string{"group1"},
		},
		{
			name: "nested struct with multiple NamedFlagSets",
			input: struct {
				Outer *Named
				Inner struct {
					NamedFlagSet *Named
				}
			}{
				Outer: func() *Named {
					n := NewNamed()
					n.FlagSet("outer")
					return n
				}(),
				Inner: struct {
					NamedFlagSet *Named
				}{
					NamedFlagSet: func() *Named {
						n := NewNamed()
						n.FlagSet("inner")
						return n
					}(),
				},
			},
			wantOrder: []string{"outer", "inner"},
		},
		{
			name: "nil NamedFlagSet fields skipped gracefully",
			input: struct {
				First  *Named
				Second *Named
				Third  *Named
			}{
				First: func() *Named {
					n := NewNamed()
					n.FlagSet("first")
					return n
				}(),
				Second: nil,
				Third: func() *Named {
					n := NewNamed()
					n.FlagSet("third")
					return n
				}(),
			},
			wantOrder: []string{"first", "third"},
		},
		{
			name: "pointer to struct with NamedFlagSet",
			input: &struct {
				NamedFlagSet *Named
			}{
				NamedFlagSet: func() *Named {
					n := NewNamed()
					n.FlagSet("pointed")
					return n
				}(),
			},
			wantOrder: []string{"pointed"},
		},
		{
			name: "deeply nested structs",
			input: struct {
				Level1 struct {
					Level2 struct {
						Level3 struct {
							NamedFlagSet *Named
						}
					}
				}
			}{
				Level1: struct {
					Level2 struct {
						Level3 struct {
							NamedFlagSet *Named
						}
					}
				}{
					Level2: struct {
						Level3 struct {
							NamedFlagSet *Named
						}
					}{
						Level3: struct {
							NamedFlagSet *Named
						}{
							NamedFlagSet: func() *Named {
								n := NewNamed()
								n.FlagSet("deep")
								return n
							}(),
						},
					},
				},
			},
			wantOrder: []string{"deep"},
		},
		{
			name: "pointer fields traversed",
			input: struct {
				Ptr *struct {
					NamedFlagSet *Named
				}
			}{
				Ptr: &struct {
					NamedFlagSet *Named
				}{
					NamedFlagSet: func() *Named {
						n := NewNamed()
						n.FlagSet("via-ptr")
						return n
					}(),
				},
			},
			wantOrder: []string{"via-ptr"},
		},
		{
			name: "nil pointer fields handled",
			input: struct {
				Ptr *struct {
					NamedFlagSet *Named
				}
				Direct *Named
			}{
				Ptr: nil,
				Direct: func() *Named {
					n := NewNamed()
					n.FlagSet("direct")
					return n
				}(),
			},
			wantOrder: []string{"direct"},
		},
		{
			name: "empty struct returns empty Named",
			input: struct {
			}{},
			wantOrder: nil,
		},
		{
			name: "struct with no NamedFlagSet returns empty Named",
			input: struct {
				Name  string
				Value int
			}{
				Name:  "test",
				Value: 42,
			},
			wantOrder: nil,
		},
		{
			name: "merges duplicate group names",
			input: struct {
				First  *Named
				Second *Named
			}{
				First: func() *Named {
					n := NewNamed()
					fs := n.FlagSet("shared")
					fs.String("flag1", "", "first flag")
					return n
				}(),
				Second: func() *Named {
					n := NewNamed()
					fs := n.FlagSet("shared")
					fs.String("flag2", "", "second flag")
					return n
				}(),
			},
			wantOrder: []string{"shared"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CollectNamedFlagSets(tt.input)

			require.NotNil(t, got)

			if d := cmp.Diff(tt.wantOrder, got.Order); d != "" {
				t.Errorf("CollectNamedFlagSets() order mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestCollectNamedFlagSets_MergesFlagsCorrectly(t *testing.T) {
	// verify that flags from merged groups are combined
	input := struct {
		First  *Named
		Second *Named
	}{
		First: func() *Named {
			n := NewNamed()
			fs := n.FlagSet("test")
			fs.String("flag1", "default1", "first flag")
			return n
		}(),
		Second: func() *Named {
			n := NewNamed()
			fs := n.FlagSet("test")
			fs.String("flag2", "default2", "second flag")
			return n
		}(),
	}

	got := CollectNamedFlagSets(input)

	require.Len(t, got.Order, 1)
	require.Equal(t, "test", got.Order[0])

	fs := got.FlagSets["test"]
	require.NotNil(t, fs)

	flag1 := fs.Lookup("flag1")
	require.NotNil(t, flag1, "flag1 should exist in merged flagset")
	require.Equal(t, "default1", flag1.DefValue)

	flag2 := fs.Lookup("flag2")
	require.NotNil(t, flag2, "flag2 should exist in merged flagset")
	require.Equal(t, "default2", flag2.DefValue)
}

type selfReferencing struct {
	NamedFlagSet *Named
	Self         *selfReferencing
}

func TestCollectNamedFlagSets_HandlesCycles(t *testing.T) {
	// create a circular reference
	a := &selfReferencing{
		NamedFlagSet: func() *Named {
			n := NewNamed()
			n.FlagSet("cyclic")
			return n
		}(),
	}
	a.Self = a

	// this should not hang or panic
	got := CollectNamedFlagSets(a)

	require.NotNil(t, got)
	require.Equal(t, []string{"cyclic"}, got.Order)
}
