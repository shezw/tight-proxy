package tray

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/getlantern/systray"
	"github.com/shezw/tight-proxy/internal/icon"
	rtpkg "github.com/shezw/tight-proxy/internal/runtime"
)

func Run(rt *rtpkg.Runtime, controlURL string) {
	onReady := func() {
		setTrayIcon()
		systray.SetTooltip("tight-proxy")
		open := systray.AddMenuItem("Open Control Panel", "Open tight-proxy control panel")
		start := systray.AddMenuItem("Start Proxy", "Start local proxy")
		stop := systray.AddMenuItem("Stop Proxy", "Stop local proxy")
		systray.AddSeparator()
		quit := systray.AddMenuItem("Quit", "Quit tight-proxy")
		go func() {
			for {
				select {
				case <-open.ClickedCh:
					_ = openBrowser(controlURL)
				case <-start.ClickedCh:
					_, _ = rt.StartProxy()
				case <-stop.ClickedCh:
					_, _ = rt.StopProxy()
				case <-quit.ClickedCh:
					_, _ = rt.StopProxy()
					systray.Quit()
					return
				}
			}
		}()
	}
	systray.Run(onReady, func() {})
}

func setTrayIcon() {
	switch runtime.GOOS {
	case "darwin":
		systray.SetTemplateIcon(icon.LightningCircleTemplatePNG(), icon.LightningCirclePNG())
	case "windows":
		systray.SetIcon(icon.LightningCircleICO())
	default:
		systray.SetIcon(icon.LightningCirclePNG())
		systray.SetTitle("tight-proxy")
	}
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}
	return nil
}
