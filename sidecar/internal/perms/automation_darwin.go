package perms

// #cgo LDFLAGS: -framework CoreServices
// #include <CoreServices/CoreServices.h>
// #include <string.h>
// #include <stdlib.h>
// static OSStatus pcs_automation(const char *bid, Boolean ask) {
//   AEAddressDesc t; OSStatus e = AECreateDesc(typeApplicationBundleID, bid, strlen(bid), &t);
//   if (e != noErr) return e;
//   e = AEDeterminePermissionToAutomateTarget(&t, typeWildCard, typeWildCard, ask);
//   AEDisposeDesc(&t); return e;
// }
import "C"

import (
	"os/exec"
	"time"
	"unsafe"
)

func automation(bundleID string, ask bool) Status {
	cs := C.CString(bundleID)
	defer C.free(unsafe.Pointer(cs))
	var b C.Boolean
	if ask {
		b = 1
	}
	return StatusFromOSStatus(int(C.pcs_automation(cs, b)))
}

// CheckAutomation reports the Automation permission for bundleID without prompting.
func CheckAutomation(bundleID string) Status {
	return automation(bundleID, false)
}

// RequestAutomation asks macOS for Automation permission for bundleID. If the
// target app is not running it is launched in the background first (up to 3s
// wait). With ask=true the call may block until the user answers the system
// prompt, so call it only from `pcs perm request`, never from a hook.
func RequestAutomation(bundleID string) Status {
	if CheckAutomation(bundleID) == Unknown {
		_ = exec.Command("open", "-g", "-b", bundleID).Run()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			time.Sleep(200 * time.Millisecond)
			if CheckAutomation(bundleID) != Unknown {
				break
			}
		}
	}
	return automation(bundleID, true)
}
