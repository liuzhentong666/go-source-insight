package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func aclScript(t *testing.T, directory, script string) []byte {
	t.Helper()
	command := exec.Command(filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe"), "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", `try { Import-Module (Join-Path $PSHOME 'Modules\Microsoft.PowerShell.Security\Microsoft.PowerShell.Security.psd1') -ErrorAction Stop; `+script+` } catch { [Console]::Error.WriteLine($_.Exception.GetType().Name); exit 2 }`)
	command.Env = append(os.Environ(), "GOSI_ACL_TEST_DIRECTORY="+directory)
	output, err := command.Output()
	if err != nil {
		if failure, ok := err.(*exec.ExitError); ok {
			t.Fatalf("ACL fixture operation failed: %s", failure.Stderr)
		}
		t.Fatal("ACL fixture operation failed")
	}
	return output
}
func TestExistingExplicitBroadACLRejectedWithoutChanges(t *testing.T) {
	root := t.TempDir()
	source := fixtureDir(t, root, "source")
	state := filepath.Join(root, "state")
	if err := configure(17, 29, source, state, nil, false); err != nil {
		t.Fatal(err)
	}
	_, before, _ := committed(t, state)
	if output, err := exec.Command(filepath.Join(os.Getenv("SystemRoot"), "System32", "icacls.exe"), state, "/grant", "*S-1-1-0:(R)").CombinedOutput(); err != nil {
		_ = output
		t.Fatal("Cannot prepare explicit Everyone ACL fixture")
	}
	read := `(Get-Acl -LiteralPath $env:GOSI_ACL_TEST_DIRECTORY).GetSecurityDescriptorSddlForm([System.Security.AccessControl.AccessControlSections]::All)`
	beforeACL := aclScript(t, state, read)
	if err := configure(31, 47, source, state, nil, false); err == nil {
		t.Fatal("Explicit Everyone ACL was accepted")
	}
	afterACL := aclScript(t, state, read)
	if !bytes.Equal(beforeACL, afterACL) {
		t.Fatal("An existing directory ACL was changed without explicit authorization")
	}
	if after, _ := os.ReadFile(filepath.Join(state, "source-bindings.compose.json")); !bytes.Equal(before, after) {
		t.Fatal("Unsafe existing ACL caused committed mapping to change")
	}
}
