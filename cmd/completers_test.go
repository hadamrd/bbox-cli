package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

// With bx unset (no live router), every completer should return an error
// directive rather than crash. Cobra will then simply omit completions.
func TestCompleters_ReturnErrorWhenClientNil(t *testing.T) {
	bx = nil
	cases := []struct {
		name string
		fn   func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective)
	}{
		{"completeHost", completeHost},
		{"completeNATRule", completeNATRule},
		{"completeNATDescription", completeNATDescription},
		{"completeFirewallRuleID", completeFirewallRuleID},
	}
	for _, tc := range cases {
		out, dir := tc.fn(nil, nil, "")
		if out != nil {
			t.Errorf("%s: expected nil suggestions, got %v", tc.name, out)
		}
		if dir != cobra.ShellCompDirectiveError {
			t.Errorf("%s: expected ShellCompDirectiveError, got %v", tc.name, dir)
		}
	}
}

// When a positional arg has already been supplied, completers should refuse
// further completion (NoFileComp) rather than re-suggesting.
func TestCompleters_NoSecondArgCompletion(t *testing.T) {
	out, dir := completeHost(nil, []string{"already-set"}, "")
	if out != nil {
		t.Errorf("expected nil suggestions after first arg; got %v", out)
	}
	if dir != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected NoFileComp; got %v", dir)
	}
}
