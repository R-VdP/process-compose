package tui

import (
	"fmt"
	"github.com/f1bonacc1/process-compose/src/config"
	"github.com/f1bonacc1/process-compose/src/updater"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/f1bonacc1/process-compose/src/app"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type FullScrState int

const (
	LogFull     FullScrState = 0
	ProcFull                 = 1
	LogProcHalf              = 2
)

const (
	PageMain   = "main"
	PageDialog = "dialog"
)

var pcv *pcView = nil
var scFiles = []string{
	"shortcuts.yaml",
	"shortcuts.yml",
}

type pcView struct {
	procTable    *tview.Table
	statTable    *tview.Table
	appView      *tview.Application
	logsText     *LogView
	statusText   *tview.TextView
	helpText     *tview.TextView
	pages        *tview.Pages
	procNames    []string
	version      string
	logFollow    bool
	fullScrState FullScrState
	loggedProc   string
	shortcuts    ShortCuts
}

func newPcView(version string, logLength int) *pcView {
	//_ = pv.shortcuts.loadFromFile("short-cuts-new.yaml")
	pv := &pcView{
		appView:      tview.NewApplication(),
		logsText:     NewLogView(logLength),
		statusText:   tview.NewTextView().SetDynamicColors(true),
		procNames:    app.PROJ.GetLexicographicProcessNames(),
		version:      version,
		logFollow:    true,
		fullScrState: LogProcHalf,
		helpText:     tview.NewTextView().SetDynamicColors(true),
		loggedProc:   "",
		shortcuts:    getDefaultActions(),
	}
	pv.loadShortcuts()
	pv.procTable = pv.createProcTable()
	pv.statTable = pv.createStatTable()
	pv.updateHelpTextView()
	pv.pages = tview.NewPages().
		AddPage(PageMain, pv.createGrid(pv.fullScrState), true, true)

	pv.appView.SetRoot(pv.pages, true).EnableMouse(true).SetInputCapture(pv.onAppKey)
	if len(pv.procNames) > 0 {
		name := pv.procNames[0]
		pv.logsText.SetTitle(name)
		pv.followLog(name)
	}
	return pv
}

func (pv *pcView) loadShortcuts() {
	for _, path := range scFiles {
		if err := pv.shortcuts.loadFromFile(filepath.Join(config.ProcCompHome(), path)); err == nil {
			break
		}
	}
}

func (pv *pcView) onAppKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case pv.shortcuts.ShortCutKeys[ActionQuit].key:
		pv.terminateAppView()
	case pv.shortcuts.ShortCutKeys[ActionLogScreen].key:
		if pv.fullScrState == LogFull {
			pv.fullScrState = LogProcHalf
		} else {
			pv.fullScrState = LogFull
		}
		pv.appView.SetRoot(pv.createGrid(pv.fullScrState), true)
		pv.updateHelpTextView()
	case pv.shortcuts.ShortCutKeys[ActionFollowLog].key:
		pv.toggleLogFollow()
	case pv.shortcuts.ShortCutKeys[ActionWrapLog].key:
		pv.logsText.ToggleWrap()
		pv.updateHelpTextView()
	case pv.shortcuts.ShortCutKeys[ActionProcessScreen].key:
		if pv.fullScrState == ProcFull {
			pv.fullScrState = LogProcHalf
		} else {
			pv.fullScrState = ProcFull
		}
		pv.appView.SetRoot(pv.createGrid(pv.fullScrState), true)
		pv.onProcRowSpanChange()
		pv.updateHelpTextView()
	case tcell.KeyCtrlC:
		pv.terminateAppView()
	case pv.shortcuts.ShortCutKeys[ActionProcessInfo].key:
		pv.showInfo()
	default:
		return event
	}
	return nil
}

func (pv *pcView) terminateAppView() {

	m := tview.NewModal().
		SetText("Are you sure you want to quit?\nThis will terminate all the running processes.").
		AddButtons([]string{"Quit", "Cancel"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Quit" {
				go pv.handleShutDown()
			}
			pv.pages.SwitchToPage(PageMain)
			pv.pages.RemovePage(PageDialog)
		})
	// Display and focus the dialog
	pv.pages.AddPage(PageDialog, createPage(m, 50, 50), true, true)
}

func (pv *pcView) showError(errMessage string) {
	m := tview.NewModal().
		SetText(fmt.Sprintf("Error: [white:red]%s[-:-:-]", errMessage)).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			pv.pages.SwitchToPage(PageMain)
			pv.pages.RemovePage(PageDialog)
		})

	pv.pages.AddPage(PageDialog, createPage(m, 50, 50), true, true)
}

func (pv *pcView) showInfo() {
	name := pv.getSelectedProcName()
	info, err := app.PROJ.GetProcessInfo(name)
	if err != nil {
		pv.showError(err.Error())
		return
	}
	pv.showProcInfo(info)
}

func (pv *pcView) handleShutDown() {
	pv.statTable.SetCell(0, 2, tview.NewTableCell("Shutting Down...").
		SetSelectable(false).
		SetAlign(tview.AlignCenter).
		SetExpansion(0).
		SetTextColor(tcell.ColorWhite).
		SetBackgroundColor(tcell.ColorRed))
	app.PROJ.ShutDownProject()
	time.Sleep(time.Second)
	pv.appView.Stop()

}

func (pv *pcView) fillTableData() {
	if app.PROJ == nil {
		return
	}

	for r, name := range pv.procNames {
		state := app.PROJ.GetProcessState(name)
		if state == nil {
			return
		}
		pv.procTable.SetCell(r+1, 0, tview.NewTableCell(strconv.Itoa(state.Pid)).SetAlign(tview.AlignRight).SetExpansion(0).SetTextColor(tcell.ColorLightSkyBlue))
		pv.procTable.SetCell(r+1, 1, tview.NewTableCell(state.Name).SetAlign(tview.AlignLeft).SetExpansion(1).SetTextColor(tcell.ColorLightSkyBlue))
		pv.procTable.SetCell(r+1, 2, tview.NewTableCell(state.Status).SetAlign(tview.AlignLeft).SetExpansion(1).SetTextColor(tcell.ColorLightSkyBlue))
		pv.procTable.SetCell(r+1, 3, tview.NewTableCell(state.SystemTime).SetAlign(tview.AlignLeft).SetExpansion(1).SetTextColor(tcell.ColorLightSkyBlue))
		pv.procTable.SetCell(r+1, 4, tview.NewTableCell(state.Health).SetAlign(tview.AlignLeft).SetExpansion(1).SetTextColor(tcell.ColorLightSkyBlue))
		pv.procTable.SetCell(r+1, 5, tview.NewTableCell(strconv.Itoa(state.Restarts)).SetAlign(tview.AlignRight).SetExpansion(0).SetTextColor(tcell.ColorLightSkyBlue))
		pv.procTable.SetCell(r+1, 6, tview.NewTableCell(strconv.Itoa(state.ExitCode)).SetAlign(tview.AlignRight).SetExpansion(0).SetTextColor(tcell.ColorLightSkyBlue))
	}
}

func (pv *pcView) getSelectedProcName() string {
	if pv.procTable == nil {
		return ""
	}
	row, _ := pv.procTable.GetSelection()
	if row > 0 && row <= len(pv.procNames) {
		return pv.procNames[row-1]
	}
	return ""
}

func (pv *pcView) onTableSelectionChange(row, column int) {
	name := pv.getSelectedProcName()
	if len(name) == 0 {
		return
	}
	pv.logsText.SetBorder(true).SetTitle(name)
	pv.unFollowLog()
	pv.followLog(name)
	if !pv.logFollow {
		// call follow and unfollow to update the buffer and stop following
		// in case the following is disabled
		pv.unFollowLog()
	}
}

func (pv *pcView) onProcRowSpanChange() {
	if pv.fullScrState == ProcFull && pv.logFollow {
		pv.stopFollowLog()
	}
}

func (pv *pcView) toggleLogFollow() {
	if pv.logFollow {
		pv.stopFollowLog()
	} else {
		name := pv.getSelectedProcName()
		pv.startFollowLog(name)
	}
}

func (pv *pcView) startFollowLog(name string) {
	pv.logFollow = true
	pv.followLog(name)
	go pv.updateLogs()
	pv.updateHelpTextView()
}

func (pv *pcView) stopFollowLog() {
	pv.logFollow = false
	pv.unFollowLog()
	pv.updateHelpTextView()
}

func (pv *pcView) followLog(name string) {
	pv.loggedProc = name
	pv.logsText.Clear()
	_ = app.PROJ.WithProcesses([]string{name}, func(process app.ProcessConfig) error {
		pv.logsText.useAnsi = !process.DisableAnsiColors
		return nil
	})
	app.PROJ.GetLogsAndSubscribe(name, pv.logsText)
}

func (pv *pcView) unFollowLog() {
	if pv.loggedProc != "" {
		app.PROJ.UnSubscribeLogger(pv.loggedProc)
	}
	pv.logsText.Flush()
}

func (pv *pcView) createProcTable() *tview.Table {
	table := tview.NewTable().SetBorders(false).SetSelectable(true, false)
	//pv.fillTableData()
	table.Select(1, 1).SetFixed(1, 0).SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case pv.shortcuts.ShortCutKeys[ActionProcessStop].key:
			name := pv.getSelectedProcName()
			app.PROJ.StopProcess(name)
		case pv.shortcuts.ShortCutKeys[ActionProcessStart].key:
			name := pv.getSelectedProcName()
			app.PROJ.StartProcess(name)
		case pv.shortcuts.ShortCutKeys[ActionProcessRestart].key:
			name := pv.getSelectedProcName()
			app.PROJ.RestartProcess(name)
		}
		return event
	})
	columns := []string{
		"PID", "NAME", "STATUS", "AGE", "READINESS", "RESTARTS", "EXIT CODE",
	}
	for i := 0; i < len(columns); i++ {
		expan := 1
		align := tview.AlignLeft
		switch columns[i] {
		case
			"PID":
			expan = 0
		case
			"RESTARTS",
			"EXIT CODE":
			align = tview.AlignRight
		}

		table.SetCell(0, i, tview.NewTableCell(columns[i]).
			SetSelectable(false).SetExpansion(expan).SetAlign(align))
	}
	table.SetSelectionChangedFunc(pv.onTableSelectionChange)
	return table
}

func (pv *pcView) createStatTable() *tview.Table {
	table := tview.NewTable().SetBorders(false).SetSelectable(false, false)

	table.SetCell(0, 0, tview.NewTableCell("Version:").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	table.SetCell(0, 1, tview.NewTableCell(pv.version).SetSelectable(false).SetExpansion(1))

	table.SetCell(1, 0, tview.NewTableCell("Hostname:").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	hostname, err := os.Hostname()
	if err != nil {
		hostname = err.Error()
	}
	table.SetCell(1, 1, tview.NewTableCell(hostname).SetSelectable(false).SetExpansion(1))

	table.SetCell(2, 0, tview.NewTableCell("Processes:").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	table.SetCell(2, 1, tview.NewTableCell(strconv.Itoa(len(pv.procNames))).SetSelectable(false).SetExpansion(1))
	table.SetCell(0, 2, tview.NewTableCell("").SetSelectable(false).SetExpansion(0))

	table.SetCell(0, 3, tview.NewTableCell("Process Compose 🔥").
		SetSelectable(false).
		SetAlign(tview.AlignRight).
		SetExpansion(1).
		SetTextColor(tcell.ColorYellow))
	return table
}

func (pv *pcView) updateHelpTextView() {
	logScrBool := pv.fullScrState != LogFull
	procScrBool := pv.fullScrState != ProcFull
	pv.helpText.Clear()
	fmt.Fprintf(pv.helpText, "%s ", "[lightskyblue:]LOGS:[-:-:-]")
	pv.shortcuts.ShortCutKeys[ActionLogScreen].writeToggleButton(pv.helpText, logScrBool)
	pv.shortcuts.ShortCutKeys[ActionFollowLog].writeToggleButton(pv.helpText, !pv.logFollow)
	pv.shortcuts.ShortCutKeys[ActionWrapLog].writeToggleButton(pv.helpText, !pv.logsText.IsWrapOn())
	fmt.Fprintf(pv.helpText, "%s ", "[lightskyblue::b]PROCESS:[-:-:-]")
	pv.shortcuts.ShortCutKeys[ActionProcessInfo].writeButton(pv.helpText)
	pv.shortcuts.ShortCutKeys[ActionProcessStart].writeButton(pv.helpText)
	pv.shortcuts.ShortCutKeys[ActionProcessScreen].writeToggleButton(pv.helpText, procScrBool)
	pv.shortcuts.ShortCutKeys[ActionProcessStop].writeButton(pv.helpText)
	pv.shortcuts.ShortCutKeys[ActionProcessRestart].writeButton(pv.helpText)
	pv.shortcuts.ShortCutKeys[ActionQuit].writeButton(pv.helpText)
}

func (pv *pcView) createGrid(fullScrState FullScrState) *tview.Grid {

	grid := tview.NewGrid().
		SetRows(3, 0, 0, 1).
		//SetColumns(30, 0, 30).
		SetBorders(true).
		AddItem(pv.statTable, 0, 0, 1, 1, 0, 0, false).
		AddItem(pv.helpText, 3, 0, 1, 1, 0, 0, false)

	switch fullScrState {
	case LogFull:
		grid.AddItem(pv.logsText, 1, 0, 2, 1, 0, 0, true)
	case ProcFull:
		grid.AddItem(pv.procTable, 1, 0, 2, 1, 0, 0, true)
	case LogProcHalf:
		grid.AddItem(pv.procTable, 1, 0, 1, 1, 0, 0, true)
		grid.AddItem(pv.logsText, 2, 0, 1, 1, 0, 0, false)
	}

	grid.SetTitle("Process Compose")
	return grid
}

func (pv *pcView) updateTable() {
	for {
		time.Sleep(1000 * time.Millisecond)
		pv.appView.QueueUpdateDraw(func() {
			pv.fillTableData()
		})
	}
}

func (pv *pcView) updateLogs() {
	for {
		pv.appView.QueueUpdateDraw(func() {
			pv.logsText.Flush()
		})
		if !pv.logFollow {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func (pv *pcView) runOnce() {
	version, err := updater.GetLatestReleaseName()
	if err != nil {
		return
	}
	if pv.version != version {
		pv.showUpdateAvailable(version)
	}
}

func (pv *pcView) showUpdateAvailable(version string) {
	pv.appView.QueueUpdateDraw(func() {
		pv.statTable.GetCell(0, 1).SetText(fmt.Sprintf("%s  [green:]%s[-:-:-]", pv.version, version))
	})
}

func SetupTui(version string, logLength int) {

	pv := newPcView(version, logLength)

	go pv.updateTable()
	go pv.updateLogs()
	go pv.runOnce()

	pcv = pv
	if err := pv.appView.Run(); err != nil {
		panic(err)
	}
}

func Stop() {
	if pcv != nil {
		pcv.handleShutDown()
	}
}
