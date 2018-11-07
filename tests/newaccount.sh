#!/usr/bin/expect

spawn ./box --config .devconfig/.box-1.yaml --wallet_dir .devconfig/ws1/box_keystore wallet newaccount
expect "Please Input Your Passphrase"
send "zaq12wsx\r"
expect eof
