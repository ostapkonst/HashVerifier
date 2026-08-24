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

// dragDestUnset clears a widget's drop destination so drag-and-drop events propagate up to a parent handler.
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

// GTK_IS_* macros are used directly to avoid any ordering dependency on GObject type-system initialization: the macros call gtk_*_get_type() which registers the type on first use, matching gotk3's pattern.
func isInputWidgetC(w *C.GtkWidget) bool {
	return C.hv_is_entry(w) != 0
}

func isContainerC(w *C.GtkWidget) bool {
	return C.hv_is_container(w) != 0
}

func collectInputWidgets(root gtk.IWidget) []gtk.IWidget {
	var result []gtk.IWidget
	collectInputWidgetsInto(root, &result)
	return result
}

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
