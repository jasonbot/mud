#! /bin/bash

OSSES="darwin linux windows"
ARCHES="amd64"
ZIPPREFIX=`date +%Y-%m-%d`

for OS in $OSSES
do
	for ARCH in $ARCHES
	do
		echo app-$OS-$ARCH
		OUTDIRNAME=mud-$OS-$ARCH
		mkdir $OUTDIRNAME
		ENABLE_CGO=""

		INFILE=cmd/mud.go
		OUTITEM=$OUTDIRNAME/mud
		if [ "z$OS" = "zlinux" ]
		then
			echo "Linux"
		elif [ "z$OS" = "zwindows" ]
		then
			OUTITEM=$OUTITEM.exe
		elif [ "z$OS" = "zdarwin" ]
		then
			INFILE=cmd/mud-ui.go
		fi

		# brew install mingw-w64
		GOOS=$OS GOARCH=$ARCH go build -o $OUTITEM $INFILE

		cp *.json *.txt $OUTDIRNAME
		if [ "z$OS" = "zwindows" ]
		then
			zip mud-$ZIPPREFIX-$OS-$ARCH.zip $OUTDIRNAME/*
		else
			tar cvzf mud-$ZIPPREFIX-$OS-$ARCH.tgz $OUTDIRNAME
		fi
		rm -rf $OUTDIRNAME
	done
done
