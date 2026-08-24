package app

// #cgo pkg-config: gtk+-3.0
// #include <gtk/gtk.h>
//
// #ifndef HV_DND_HELPERS
// #define HV_DND_HELPERS
// static gboolean hv_is_entry(GtkWidget *w) { return GTK_IS_ENTRY(w) || GTK_IS_TEXT_VIEW(w); }
// static gboolean hv_is_container(GtkWidget *w) { return GTK_IS_CONTAINER(w); }
// #endif
import "C"

import (
	"unsafe"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// dragDestUnset clears a widget's drop destination so drag-and-drop events
// propagate up to a parent widget that has its own handler. It mirrors
// gotk3's own DragDestSet wrapper.
func dragDestUnset(w gtk.IWidget) {
	if w == nil {
		return
	}
	widget := w.ToWidget()
	if widget == nil || widget.GObject == nil {
		return
	}
	C.gtk_drag_dest_unset((*C.GtkWidget)(unsafe.Pointer(widget.GObject)))
}

// isInputWidgetC reports whether the C-level GtkWidget is a text input
// widget (GtkEntry and its subclasses like GtkSearchEntry, GtkSpinButton,
// or GtkTextView) that has built-in drag-and-drop handling.
//
// Using GTK_IS_* macros directly (rather than g_type_from_name in a
// package var) avoids any ordering dependency on GObject type-system
// initialization: the macros call gtk_*_get_type() which registers the
// type on first use, matching gotk3's own pattern in gtk.init().
func isInputWidgetC(w *C.GtkWidget) bool {
	return C.hv_is_entry(w) != 0
}

// isContainerC reports whether the C-level GtkWidget is a GtkContainer
// (i.e. can have children to recurse into during the tree walk).
func isContainerC(w *C.GtkWidget) bool {
	return C.hv_is_container(w) != 0
}

// collectInputWidgets walks the widget tree starting at root and returns
// every input widget (GtkEntry, GtkSearchEntry, GtkTextView, ...) found.
// The caller is responsible for acting on the returned list.
func collectInputWidgets(root gtk.IWidget) []gtk.IWidget {
	var result []gtk.IWidget
	collectInputWidgetsInto(root, &result)
	return result
}

// collectInputWidgetsInto recursively walks the widget tree, appending every
// input widget found to result. Containers are traversed to reach nested
// inputs (e.g. an entry inside a notebook page).
func collectInputWidgetsInto(w gtk.IWidget, result *[]gtk.IWidget) {
	if w == nil {
		return
	}
	widget := w.ToWidget()
	if widget == nil || widget.GObject == nil {
		return
	}

	cWidget := (*C.GtkWidget)(unsafe.Pointer(widget.GObject))
	if isInputWidgetC(cWidget) {
		*result = append(*result, w)
	}

	if !isContainerC(cWidget) {
		return
	}

	children := C.gtk_container_get_children((*C.GtkContainer)(unsafe.Pointer(cWidget)))
	if children == nil {
		return
	}
	defer C.g_list_free(children)

	for c := children; c != nil; c = c.next {
		if c.data == nil {
			continue
		}
		childWidget := (*C.GtkWidget)(unsafe.Pointer(c.data))
		switch {
		case isInputWidgetC(childWidget):
			if iw := widgetFromC(childWidget); iw != nil {
				*result = append(*result, iw)
			}
		case isContainerC(childWidget):
			if iw := widgetFromC(childWidget); iw != nil {
				collectInputWidgetsInto(iw, result)
			}
		}
	}
}

// widgetFromC wraps a raw GtkWidget pointer as gtk.IWidget using gotk3's
// public Cast mechanism. The returned widget owns a reference (via glib.Take).
func widgetFromC(w *C.GtkWidget) gtk.IWidget {
	if w == nil {
		return nil
	}
	obj := glib.Take(unsafe.Pointer(w))
	widget := &gtk.Widget{InitiallyUnowned: glib.InitiallyUnowned{Object: obj}}
	iw, err := widget.Cast()
	if err != nil {
		return nil
	}
	return iw
}
