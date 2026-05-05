package notify

import (
	"encoding/base64"
	"fmt"
	"os/exec"
	"syscall"
)

// Show displays a Windows 10/11 toast notification using the WinRT API.
// It fires asynchronously and does not block the caller.
func Show(title, body string) {
	titleB64 := base64.StdEncoding.EncodeToString([]byte(title))
	bodyB64 := base64.StdEncoding.EncodeToString([]byte(body))

	ps := fmt.Sprintf(`$t=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('%s'));$b=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('%s'));$et=[Security.SecurityElement]::Escape($t);$eb=[Security.SecurityElement]::Escape($b);[Windows.UI.Notifications.ToastNotificationManager,Windows.UI.Notifications,ContentType=WindowsRuntime]|Out-Null;[Windows.Data.Xml.Dom.XmlDocument,Windows.Data.Xml.Dom,ContentType=WindowsRuntime]|Out-Null;$x="<toast><visual><binding template='ToastText02'><text id='1'>$et</text><text id='2'>$eb</text></binding></visual></toast>";$d=New-Object Windows.Data.Xml.Dom.XmlDocument;$d.LoadXml($x);[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('WailsChat').Show([Windows.UI.Notifications.ToastNotification]::new($d))`, titleB64, bodyB64)

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", ps)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Start() // fire-and-forget
}
