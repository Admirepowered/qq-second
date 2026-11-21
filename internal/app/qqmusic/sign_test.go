package qqmusic

import "testing"

func TestZZCSign(t *testing.T) {
	sign := zzcSign([]byte(`{"comm":{"ct":19,"cv":1,"tmeAppID":"qqmusic"}}`))
	want := "zzc667ce25ux835nhknj1bysb9mzdj7witfce03a5004"
	if sign != want {
		t.Fatalf("zzcSign = %s, want %s", sign, want)
	}
}

func TestQRCDecrypt(t *testing.T) {
	encrypted := "3c80fea4c8965b324d9d7f9b0778e5be0374013221f3c86fdbab3be5929b9320ea64d4ea7f2fa40a"
	got := qrcDecrypt(encrypted)
	if got != "[ti:test][00:00.00]你好世界" {
		t.Fatalf("qrcDecrypt = %q", got)
	}
}
