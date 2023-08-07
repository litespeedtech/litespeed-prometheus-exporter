#!/bin/sh

OSNAMEVER=UNKNOWN
OSNAME=
OSVER=
DLCMD=
LSPHPVER=74
MYIP=

#script start here
cd `dirname "$0"`
. ./functions.sh
if [ $? != 0 ] ; then
    echo "[ERROR] Can not include 'functions.sh'."
    exit 1
fi
if [ "$#" != "0" ]; then
    SERVER_ADDR=$1
fi

#If install.sh in admin/misc, need to change directory
LSINSTALL_DIR=`dirname "$0"`
#cd $LSINSTALL_DIR/

init
install_dir
stopExporter
./rc-uninst.sh
rm -rf $LSWS_HOME

echo
echo -e "\033[38;5;148mLiteSpeed Web Server Prometheus Exporter uninstalled\033[39m"
echo


