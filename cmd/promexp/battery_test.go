package main

import "testing"

func TestBatteryState(t *testing.T) {
	t.Log(batteryState.val(11))
	t.Log(batteryState.val(12))
	t.Log(batteryState.val(12.3))
	t.Log(batteryState.val(12.5))
	t.Log(batteryState.val(12.9))
	t.Log(batteryState.val(13))
}
