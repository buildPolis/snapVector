//go:build linux

package gui

/*
#cgo pkg-config: gtk+-3.0
#include <stdlib.h>
#include <glib.h>
#include <gdk/gdk.h>

static void snapvector_set_linux_identity(const char *prgname, const char *programClass, const char *appName) {
	g_set_prgname(prgname);
	gdk_set_program_class(programClass);
	g_set_application_name(appName);
}
*/
import "C"

import "unsafe"

func configureLinuxProgramIdentity() {
	prgname := C.CString("snapvector")
	programClass := C.CString("Snapvector")
	appName := C.CString("SnapVector")
	defer C.free(unsafe.Pointer(prgname))
	defer C.free(unsafe.Pointer(programClass))
	defer C.free(unsafe.Pointer(appName))

	C.snapvector_set_linux_identity(prgname, programClass, appName)
}
