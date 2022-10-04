#!/bin/bash -x
if [ "$test_status" == "Failed" ]; then
    echo "Tests failed"
    exit 1
elif [ "$test_status" == "Succeeded" ]; then
    echo "Tests passed"
    exit 1
else
    echo "Unknown test status $test_status"
    exit 1
fi

