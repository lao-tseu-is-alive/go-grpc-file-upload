#!/bin/bash
#creates a 100 MB file named upload_test.bin of 1000MB
dd if=/dev/urandom of=upload_test.bin bs=1M count=1000
