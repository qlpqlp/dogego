//go:build windows || linux || (darwin && cgo)

package desktop

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"fyne.io/systray"
)

const trayUpdatePollInterval = 5 * time.Minute

func platformTraySupported() bool { return true }

func runTray(cfg TrayConfig, icon []byte) error {
	done := make(chan struct{})
	var shutdownOnce sync.Once
	doShutdown := func() {
		shutdownOnce.Do(func() {
			if cfg.OnShutdown != nil {
				cfg.OnShutdown()
			}
		})
	}

	go func() {
		// Windows: CreateWindow + GetMessage must stay on one OS thread. Without this,
		// the Go scheduler migrates the tray goroutine under IBD CPU load and the icon
		// stops responding to left/right clicks while still visible.
		runtime.LockOSThread()

		systray.Run(func() {
			if len(icon) > 0 {
				systray.SetIcon(icon)
			}
			systray.SetTitle(cfg.Title)
			upd := trayCurrentUpdate(cfg)
			initialTip := trayTooltip(cfg.Tooltip, upd)

			// Serialize menu/tooltip mutations so Hide/Show/SetTitle never race TrackPopupMenu.
			uiJobs := make(chan func(), 32)
			go func() {
				for fn := range uiJobs {
					if fn != nil {
						fn()
					}
				}
			}()
			runUI := func(fn func()) {
				if fn == nil {
					return
				}
				select {
				case uiJobs <- fn:
				default:
					go fn()
				}
			}

			go scheduleTrayTooltip(initialTip, runUI)

			verItem := systray.AddMenuItem(trayVersionLabel(cfg.Version, cfg.Network, upd), "Running DogeGo version")
			verItem.Disable()
			systray.AddSeparator()

			openItem := systray.AddMenuItem("Open Dashboard", "Open the DogeGo web UI in your browser")
			var peerItems []*systray.MenuItem
			for _, pl := range cfg.PeerLinks {
				peerItems = append(peerItems, systray.AddMenuItem(pl.Label, "Open peer network dashboard"))
			}
			consoleItem := systray.AddMenuItem("Open Console", "RPC console and activity log")
			logsItem := systray.AddMenuItem("View activity logs", "Tail the in-process activity log")
			systray.AddSeparator()

			checkItem := systray.AddMenuItem("Check for updates", "Query GitHub releases now")
			updateItem := systray.AddMenuItem("Update available", "A newer release was found")
			updateItem.Disable()
			updateItem.Hide()
			downloadItem := systray.AddMenuItem("Download update", "Save release binary to datadir/updates/")
			downloadItem.Hide()
			applyItem := systray.AddMenuItem("Install update", "Download, verify, and restart into the new version")
			applyItem.Hide()
			releaseItem := systray.AddMenuItem("View release on GitHub", "Open release notes in browser")
			releaseItem.Hide()
			dismissItem := systray.AddMenuItem("Dismiss update notice", "Hide until a newer release appears")
			dismissItem.Hide()
			systray.AddSeparator()

			quitLabel := strings.TrimSpace(cfg.QuitLabel)
			if quitLabel == "" {
				quitLabel = "Shutdown Node"
			}
			quitItem := systray.AddMenuItem(quitLabel, "Stop DogeGo and exit")

			applyTrayUpdateMenu(cfg, verItem, updateItem, downloadItem, applyItem, releaseItem, dismissItem)

			go trayUpdatePollLoop(cfg, verItem, updateItem, downloadItem, applyItem, releaseItem, dismissItem, runUI)

			// Peer dashboard opens run in their own goroutines so ShellExecute / browser
			// launch never blocks the shared menu event loop (Windows systray freezes otherwise).
			for i, item := range peerItems {
				if i >= len(cfg.PeerLinks) {
					break
				}
				url := cfg.PeerLinks[i].URL
				go trayPeerOpenLoop(item.ClickedCh, url)
			}

			// Dedicated click loops (same as peer links) so Windows never drops Open Dashboard
			// while another menu action is running.
			go trayClickLoop(openItem.ClickedCh, cfg.OnOpen)
			go trayClickLoop(consoleItem.ClickedCh, cfg.OnOpenConsole)
			go trayClickLoop(logsItem.ClickedCh, cfg.OnOpenLogs)

			go func() {
				for {
					select {
					case <-checkItem.ClickedCh:
						go func() {
							if cfg.OnCheckUpdates != nil {
								cfg.OnCheckUpdates()
							}
							runUI(func() {
								applyTrayUpdateMenu(cfg, verItem, updateItem, downloadItem, applyItem, releaseItem, dismissItem)
							})
						}()
					case <-updateItem.ClickedCh:
						go func() {
							if cfg.OnOpenRelease != nil {
								cfg.OnOpenRelease()
							}
						}()
					case <-downloadItem.ClickedCh:
						go func() {
							if cfg.OnDownloadUpdate != nil {
								path, err := cfg.OnDownloadUpdate()
								if err != nil {
									fmt.Fprintf(os.Stderr, "DogeGo tray: download update: %v\n", err)
								} else {
									fmt.Fprintf(os.Stderr, "DogeGo tray: update saved to %s\n", path)
								}
							}
							runUI(func() {
								applyTrayUpdateMenu(cfg, verItem, updateItem, downloadItem, applyItem, releaseItem, dismissItem)
							})
						}()
					case <-applyItem.ClickedCh:
						go func() {
							if cfg.OnApplyUpdate != nil {
								if err := cfg.OnApplyUpdate(); err != nil {
									fmt.Fprintf(os.Stderr, "DogeGo tray: install update: %v\n", err)
								}
							}
						}()
					case <-releaseItem.ClickedCh:
						go func() {
							if cfg.OnOpenRelease != nil {
								cfg.OnOpenRelease()
							}
						}()
					case <-dismissItem.ClickedCh:
						go func() {
							if cfg.OnDismissUpdate != nil {
								_ = cfg.OnDismissUpdate()
							}
							runUI(func() {
								applyTrayUpdateMenu(cfg, verItem, updateItem, downloadItem, applyItem, releaseItem, dismissItem)
							})
						}()
					case <-quitItem.ClickedCh:
						// Never block Quit on dual-peer shutdown (can take tens of seconds).
						go doShutdown()
						systray.Quit()
						return
					}
				}
			}()
		}, func() {
			// onExit runs from Quit / WM_DESTROY — keep it short so the native loop can finish.
			go doShutdown()
			close(done)
		})
	}()
	<-done
	return nil
}

func trayPeerOpenLoop(ch chan struct{}, url string) {
	for range ch {
		go OpenURLForce(url)
	}
}

func trayClickLoop(ch chan struct{}, fn func()) {
	if fn == nil {
		for range ch {
		}
		return
	}
	for range ch {
		go fn()
	}
}

func trayCurrentUpdate(cfg TrayConfig) TrayUpdateInfo {
	if cfg.UpdateStatus != nil {
		return cfg.UpdateStatus()
	}
	return TrayUpdateInfo{Current: cfg.Version}
}

func applyTrayUpdateMenu(cfg TrayConfig, verItem, updateItem, downloadItem, applyItem, releaseItem, dismissItem *systray.MenuItem) {
	upd := trayCurrentUpdate(cfg)
	if verItem != nil {
		verItem.SetTitle(trayVersionLabel(cfg.Version, cfg.Network, upd))
	}
	showUpdate := upd.Available && !upd.Dismissed && upd.Latest != ""
	if updateItem != nil {
		if showUpdate {
			updateItem.SetTitle("Update available: " + upd.Latest)
			updateItem.Enable()
			updateItem.Show()
		} else {
			updateItem.Hide()
		}
	}
	if downloadItem != nil {
		if showUpdate && upd.DirectDownload && upd.DownloadURL != "" {
			downloadItem.Show()
		} else {
			downloadItem.Hide()
		}
	}
	if applyItem != nil {
		if showUpdate && upd.DirectDownload && upd.DownloadURL != "" && cfg.OnApplyUpdate != nil {
			applyItem.Show()
		} else {
			applyItem.Hide()
		}
	}
	if releaseItem != nil {
		if showUpdate && upd.ReleaseURL != "" {
			releaseItem.Show()
		} else {
			releaseItem.Hide()
		}
	}
	if dismissItem != nil {
		if showUpdate {
			dismissItem.Show()
		} else {
			dismissItem.Hide()
		}
	}
}

func trayUpdatePollLoop(cfg TrayConfig, verItem, updateItem, downloadItem, applyItem, releaseItem, dismissItem *systray.MenuItem, runUI func(func())) {
	t := time.NewTicker(trayUpdatePollInterval)
	defer t.Stop()
	var lastTip string
	for range t.C {
		runUI(func() {
			applyTrayUpdateMenu(cfg, verItem, updateItem, downloadItem, applyItem, releaseItem, dismissItem)
			upd := trayCurrentUpdate(cfg)
			tip := trayTooltip(cfg.Tooltip, upd)
			if tip != lastTip {
				safeSetTooltip(tip)
				lastTip = tip
			}
		})
	}
}

// safeSetTooltip avoids Windows shell tooltip timeouts from crashing the tray loop.
func safeSetTooltip(text string) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "DogeGo systray: tooltip skipped: %v\n", r)
		}
	}()
	systray.SetTooltip(text)
}

// scheduleTrayTooltip delays the first tooltip set so the Windows notification area is ready.
func scheduleTrayTooltip(text string, runUI func(func())) {
	time.Sleep(750 * time.Millisecond)
	runUI(func() { safeSetTooltip(text) })
}
