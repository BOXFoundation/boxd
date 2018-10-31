#!/usr/bin/expect

spawn ./box --config .devconfig/.box-1.yaml wallet newaccount
expect "Please Input Your Passphrase"
send "123"
send "exit\r"
expect eof
