package widgets

import (
	"html"
	"strings"

	"github.com/gotk3/gotk3/gtk"
	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/system"
)

func ShowFlatpakSandboxWarningDialog(parent *gtk.Window) bool {
	dialog, err := gtk.DialogNew()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Flatpak warning dialog")
		return false
	}
	defer dialog.Destroy()

	dialog.SetTitle("Flatpak Sandbox Warning")
	dialog.SetTransientFor(parent)
	dialog.SetModal(true)
	dialog.SetResizable(false)
	dialog.AddButton("_Continue", gtk.RESPONSE_ACCEPT) //nolint:errcheck

	contentArea, err := dialog.GetContentArea()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get content area")
		return false
	}

	vbox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 15)
	vbox.SetMarginStart(15)
	vbox.SetMarginEnd(15)
	vbox.SetMarginTop(15)
	vbox.SetMarginBottom(10)

	messageLabel, _ := gtk.LabelNew("")
	filesystems := system.GetFlatpakFilesystems()

	var accessibleList strings.Builder

	if len(filesystems) > 0 {
		for _, fs := range filesystems {
			accessibleList.WriteString("• ")
			accessibleList.WriteString(escapePangoMarkup(fs))
			accessibleList.WriteString("\n")
		}
	} else {
		accessibleList.WriteString("Limited to specific folders\n")
	}

	messageLabel.SetMarkup(
		"<span size='large' weight='bold'>Running in Flatpak Sandbox</span>\n\n" +
			"This application is running in a sandboxed environment with limited access.\n\n" +
			"<b>Current file system access:</b>\n" +
			accessibleList.String() +
			"\n<b>To access other locations:</b>\n" +
			"Use a tool like <b>Flatseal</b> to grant additional permissions manually.",
	)
	messageLabel.SetXAlign(0)
	messageLabel.SetYAlign(0)

	suppressCheckbox, _ := gtk.CheckButtonNewWithLabel("Don't show this warning again")

	vbox.PackStart(messageLabel, true, true, 0)
	vbox.PackEnd(suppressCheckbox, false, false, 0)
	contentArea.PackStart(vbox, true, true, 0)
	dialog.ShowAll()
	response := dialog.Run()

	return response == gtk.RESPONSE_ACCEPT && suppressCheckbox.GetActive()
}

func escapePangoMarkup(s string) string {
	return html.EscapeString(s)
}
