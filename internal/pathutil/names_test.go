package pathutil

import "testing"

func TestFromNameUsesForestBranch(t *testing.T) {
	mapping, err := FromName(".forest/worktrees", "fix-login")
	if err != nil {
		t.Fatal(err)
	}
	if mapping.Identity != "fix-login" {
		t.Fatalf("identity = %q", mapping.Identity)
	}
	if mapping.Branch != "forest/fix-login" {
		t.Fatalf("branch = %q", mapping.Branch)
	}
}

func TestFromBranchUsesBranchAsIdentity(t *testing.T) {
	mapping, err := FromBranch(".forest/worktrees", "feat/login-copy")
	if err != nil {
		t.Fatal(err)
	}
	if mapping.Identity != "feat/login-copy" {
		t.Fatalf("identity = %q", mapping.Identity)
	}
	if mapping.Branch != "feat/login-copy" {
		t.Fatalf("branch = %q", mapping.Branch)
	}
}

func TestValidateIdentityRejectsTraversal(t *testing.T) {
	for _, input := range []string{"../x", "feat/../../x", "/tmp/x", "feat//x"} {
		if err := ValidateIdentity(input); err == nil {
			t.Fatalf("expected %q to be invalid", input)
		}
	}
}

func TestCheckCollisionRejectsExactAndPrefixCollisions(t *testing.T) {
	existing := []string{
		".forest/worktrees/fix-login",
		".forest/worktrees/feat/login-copy",
	}
	for _, candidate := range []string{
		".forest/worktrees/fix-login",
		".forest/worktrees/feat",
		".forest/worktrees/feat/login-copy/extra",
	} {
		if err := CheckCollision(candidate, existing); err == nil {
			t.Fatalf("expected collision for %s", candidate)
		}
	}
}

func TestContains(t *testing.T) {
	ok, err := Contains("repo/worktrees/fix-login", "repo/worktrees/fix-login/sub/dir")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected child path to be contained")
	}
	ok, err = Contains("repo/worktrees/fix-login", "repo/worktrees/other")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected sibling path not to be contained")
	}
}
