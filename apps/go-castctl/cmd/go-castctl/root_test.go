package cmd

import "testing"

func TestRootCommandStructure(t *testing.T) {
	root := NewRootCommand()
	if root.Use != "go-castctl" || root.RunE == nil {
		t.Fatalf("unexpected root command: %#v", root)
	}
	want := map[string]bool{"serve": false, "version": false, "man": false, "avatar": false}
	for _, command := range root.Commands() {
		if _, ok := want[command.Name()]; ok {
			want[command.Name()] = true
		}
		if command.Name() == "avatar" && !command.Hidden {
			t.Error("avatar command should be hidden")
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("missing %s command", name)
		}
	}
}
