package explain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fixtures are captured from a real server rather than written by hand.
// The whole difficulty here is that the plan's shape is the server's business,
// and a fixture invented to match the parser would test nothing.
func fixture(t *testing.T, name string) []byte {
	t.Helper()

	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseBuildsATreeFromTheServersOwnShape(t *testing.T) {
	plan, err := Parse(fixture(t, "mariadb-join.json"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if plan.Name != "query_block" {
		t.Errorf("the root is %q, want query_block", plan.Name)
	}

	// b is reached through nested_loop → read_sorted_file → filesort, which is
	// three levels of nesting the parser was never told about.
	table := find(plan, func(n *Node) bool { return n.Attr("table_name") == "b" })
	if table == nil {
		t.Fatalf("the scanned table is missing from the tree:\n%s", Render(plan, 80))
	}
	if got := table.Attr("access_type"); got != "ALL" {
		t.Errorf("access_type = %q, want ALL", got)
	}
	if got := table.Attr("rows"); got != "4" {
		t.Errorf("rows = %q, want 4", got)
	}

	// The filesort is a node in its own right, not a property of the table.
	if find(plan, func(n *Node) bool { return n.Name == "filesort" }) == nil {
		t.Errorf("the filesort step is missing:\n%s", Render(plan, 80))
	}
}

// A UNION nests whole query blocks inside an array. Nothing in the parser
// names union_result or query_specifications; both are just objects.
func TestParseFollowsArraysOfQueryBlocks(t *testing.T) {
	plan, err := Parse(fixture(t, "mariadb-union.json"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	var blocks int
	walk(plan, func(n *Node) {
		if n.Name == "query_block" {
			blocks++
		}
	})
	// The outer block and one per branch of the union.
	if blocks != 3 {
		t.Errorf("found %d query blocks, want 3:\n%s", blocks, Render(plan, 80))
	}
}

// A plan is only useful if the things that make a query slow stand out. Left
// among thirty other fields they have to be hunted for, which is the state
// the ordinary grid already leaves them in.
func TestTheCostlyStepsAreCalledOut(t *testing.T) {
	for _, tt := range []struct {
		file string
		want []string
	}{
		{"mariadb-join.json", []string{"full scan", "filesort"}},
		{"mariadb-groupby.json", []string{"full scan", "filesort", "temporary table"}},
	} {
		t.Run(tt.file, func(t *testing.T) {
			plan, err := Parse(fixture(t, tt.file))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			var got []string
			walk(plan, func(n *Node) { got = append(got, n.Warnings...) })

			for _, want := range tt.want {
				if !contains(got, want) {
					t.Errorf("the plan does not call out %q; found %v\n%s", want, got, Render(plan, 80))
				}
			}
		})
	}
}

// An index lookup is the case where nothing is wrong, and a plan that warns
// about everything says nothing.
func TestAnIndexLookupIsNotWarnedAbout(t *testing.T) {
	plan, err := Parse(fixture(t, "mariadb-join.json"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	lookup := find(plan, func(n *Node) bool { return n.Attr("access_type") == "eq_ref" })
	if lookup == nil {
		t.Fatal("the eq_ref lookup is missing from the tree")
	}
	if len(lookup.Warnings) != 0 {
		t.Errorf("an eq_ref lookup was flagged: %v", lookup.Warnings)
	}
}

// The acceptance for #14: readable as a tree, without reaching sideways.
func TestRenderNeverExceedsTheWidthItIsGiven(t *testing.T) {
	for _, name := range []string{"mariadb-join.json", "mariadb-union.json", "mariadb-groupby.json"} {
		for _, width := range []int{40, 60, 80, 120} {
			plan, err := Parse(fixture(t, name))
			if err != nil {
				t.Fatalf("Parse(%s) error = %v", name, err)
			}

			for _, line := range strings.Split(Render(plan, width), "\n") {
				if n := len([]rune(line)); n > width {
					t.Errorf("%s at width %d: a line is %d wide:\n%s", name, width, n, line)
				}
			}
		}
	}
}

// The condition on a scan is often the longest thing in a plan and the most
// worth reading — it is what an index would have to cover.
func TestALongConditionIsWrappedRatherThanCut(t *testing.T) {
	plan, err := Parse(fixture(t, "mariadb-join.json"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	out := Render(plan, 48)
	// The condition is longer than 48 columns, so it can only survive across
	// more than one line.
	for _, want := range []string{"b.`year` > 2001", "b.author_id is not null"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q was lost at width 48:\n%s", want, out)
		}
	}
}

// A plan that is not JSON at all — a server that answered with something
// else, or an EXPLAIN that failed — must say so rather than render an empty
// tree that looks like a query with no steps.
func TestParseRefusesWhatIsNotAPlan(t *testing.T) {
	for _, in := range []string{"", "not json", "[]", "{}", `{"nope": 1}`} {
		if _, err := Parse([]byte(in)); err == nil {
			t.Errorf("Parse(%q) returned no error", in)
		}
	}

	// Valid JSON that is simply not a plan gets its own sentence. A server
	// that answered with something else is a different problem from a server
	// that answered with nothing, and the two are told apart by the message.
	_, err := Parse([]byte(`{"nope": 1}`))
	if err == nil || !strings.Contains(err.Error(), "no query_block") {
		t.Errorf("JSON without a query_block reported %v", err)
	}
}

func find(n *Node, pred func(*Node) bool) *Node {
	var found *Node
	walk(n, func(candidate *Node) {
		if found == nil && pred(candidate) {
			found = candidate
		}
	})
	return found
}

func walk(n *Node, fn func(*Node)) {
	fn(n)
	for _, child := range n.Children {
		walk(child, fn)
	}
}

func contains(all []string, want string) bool {
	for _, s := range all {
		if s == want {
			return true
		}
	}
	return false
}

// A warning is only worth having if it means something. Two of these were
// false on the first real plans this was run against, and a plan that flags
// what cannot be fixed teaches people to stop reading the flags.
func TestTheWarningsDoNotCryWolf(t *testing.T) {
	t.Run("a sort is one warning, not two", func(t *testing.T) {
		// MariaDB wraps filesort in read_sorted_file, so warning on the node
		// name alone reported the same sort on both.
		plan, err := Parse(fixture(t, "mariadb-join.json"))
		if err != nil {
			t.Fatal(err)
		}

		var sorts int
		walk(plan, func(n *Node) {
			for _, w := range n.Warnings {
				if w == "filesort" {
					sorts++
				}
			}
		})
		if sorts != 1 {
			t.Errorf("one sort was reported %d times:\n%s", sorts, Render(plan, 80))
		}
	})

	t.Run("a union's own result is not a full scan to fix", func(t *testing.T) {
		// <union1,2> is the union reading back what it just wrote, and it is
		// access_type ALL every single time. There is no index to add.
		plan, err := Parse(fixture(t, "mariadb-union.json"))
		if err != nil {
			t.Fatal(err)
		}

		union := find(plan, func(n *Node) bool { return strings.HasPrefix(n.Name, "<") })
		if union == nil {
			t.Fatalf("the union result is missing from the tree:\n%s", Render(plan, 80))
		}
		if len(union.Warnings) != 0 {
			t.Errorf("%s was flagged %v, and nothing about it can be changed", union.Name, union.Warnings)
		}

		// The real tables underneath are still scanned, and those are the ones
		// worth saying so about.
		if find(plan, func(n *Node) bool {
			return n.Name == "dv_x_authors" && contains(n.Warnings, "full scan")
		}) == nil {
			t.Errorf("the scan of a real table went unflagged:\n%s", Render(plan, 80))
		}
	})
}
