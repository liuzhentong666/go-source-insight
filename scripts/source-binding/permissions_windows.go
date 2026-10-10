package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"syscall"
)

func isUnsafeLink(info os.FileInfo) bool {
	if info.Mode()&os.ModeSymlink != 0 {
		return true
	}
	attributes, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return ok && attributes.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
}
func restrictDirectory(directory string, newlyCreated bool) error {
	identity, err := user.Current()
	if err != nil {
		return errors.New("Cannot determine configuration-directory owner.")
	}
	script := `$ErrorActionPreference='Stop'
try {
Import-Module (Join-Path $PSHOME 'Modules\Microsoft.PowerShell.Security\Microsoft.PowerShell.Security.psd1') -ErrorAction Stop
$sid=[System.Security.Principal.SecurityIdentifier]::new($env:GOSI_BINDING_OWNER_SID)
if($env:GOSI_BINDING_NEW_DIRECTORY -eq '1') {
  $acl=[System.Security.AccessControl.DirectorySecurity]::new()
  $acl.SetOwner($sid)
  $acl.SetAccessRuleProtection($true,$false)
  $rule=[System.Security.AccessControl.FileSystemAccessRule]::new($sid,'FullControl','ContainerInherit,ObjectInherit','None','Allow')
  $acl.AddAccessRule($rule)
  Set-Acl -LiteralPath $env:GOSI_BINDING_STATE_DIRECTORY -AclObject $acl
} else {
  $acl=Get-Acl -LiteralPath $env:GOSI_BINDING_STATE_DIRECTORY
  if(-not $acl.AreAccessRulesProtected) { exit 1 }
  if($acl.GetOwner([System.Security.Principal.SecurityIdentifier]).Value -ne $sid.Value) { exit 1 }
  foreach($rule in $acl.GetAccessRules($true,$true,[System.Security.Principal.SecurityIdentifier])) {
    if($rule.AccessControlType -eq 'Allow' -and $rule.IdentityReference.Value -ne $sid.Value) { exit 1 }
  }
}
} catch { [Console]::Error.WriteLine('ACL helper operation failed: '+$_.Exception.GetType().Name); exit 2 }`
	command := exec.Command(filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe"), "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script)
	created := "0"
	if newlyCreated {
		created = "1"
	}
	command.Env = append(os.Environ(), "GOSI_BINDING_OWNER_SID="+identity.Uid, "GOSI_BINDING_STATE_DIRECTORY="+directory, "GOSI_BINDING_NEW_DIRECTORY="+created)
	if err := command.Run(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return fmt.Errorf("Configuration directory is not owner-only or cannot be protected (ACL helper exit %d). Use a new state directory or restrict it deliberately; existing permissions were not rewritten.", exitError.ExitCode())
		}
		return errors.New("Cannot launch the Windows configuration ACL helper; no binding files were written.")
	}
	return nil
}
