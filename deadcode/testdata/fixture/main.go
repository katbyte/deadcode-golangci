package main

import "fixture/lib"

func main() {
	used()
	lib.Used()
	var s speaker = thing{}
	s.speak()
}

func used() {}

// unusedTopLevel is never called and should be reported dead.
func unusedTopLevel() {}

// deadChainA and deadChainB call each other, so each has a caller — but the
// cycle is unreachable from main, which only whole-program analysis can see.
func deadChainA() { deadChainB() }

func deadChainB() { deadChainA() }

type speaker interface{ speak() }

type thing struct{}

// speak is only ever called dynamically through the speaker interface; RTA
// must keep it live.
func (thing) speak() {}

// unusedMethod is never called, but thing is converted to an interface so RTA
// conservatively keeps all its methods (reflection-callable) — not reported,
// matching cmd/deadcode.
func (thing) unusedMethod() {}

type widget struct{}

// deadMethod is never called and widget never escapes to an interface, so it
// is reported dead.
func (widget) deadMethod() {}

type marker interface{ isMarker() }

// isMarker is a marker method: dead, but deliberately not reported.
func (thing) isMarker() {}
