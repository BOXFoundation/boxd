package main

import "testing"

func TestGetIniKV(t *testing.T) {
	iniBuf := `spawn ./box --config .devconfig/.box-1.yaml wallet newaccount
Please Input Your Passphrase
Created new account: 49a7d13735b2614a65cabf4f4058783e4cecf13a
Address:b1aXG2SsUzVKePYsZxkUjHoDSYsKmyCA7wk`
	if "b1aXG2SsUzVKePYsZxkUjHoDSYsKmyCA7wk" != GetIniKV(iniBuf, "Address") {
		t.Fatalf("want: %s, got: %s", "b1aXG2SsUzVKePYsZxkUjHoDSYsKmyCA7wk",
			GetIniKV(iniBuf, "Address"))
	}
}
