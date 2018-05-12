#! /bin/bash

OSSES="darwin linux windows"
ARCHES="amd64"

for OS in $OSSES
do
	for ARCH in $ARCHES
	do
		echo app-$OS-$ARCH
		OUTDIRNAME=mud-$OS-$ARCH
		mkdir $OUTDIRNAME
		for file in cmd/*.go
		do
		fn=${file##*/}
			OUTITEM=$OUTDIRNAME/${fn%%.go}
			if [ "z$OS" = "zwindows" ]
			then
				OUTITEM=$OUTITEM.exe
			fi
			GOOS=$OS GOARCH=$ARCH go build -o $OUTITEM $file
		done
		cp *.json *.txt $OUTDIRNAME
		zip $OUTDIRNAME.zip $OUTDIRNAME/*
		rm -rf $OUTDIRNAME
	done
done
