#!/bin/sh

OSNAMEVER=UNKNOWN
OSNAME=
OSVER=
DLCMD=
LSPHPVER=74
MYIP=
NOHTTPS=0

help()
{
    printf 'LiteSpeed Prometheus Exporter installation script\n'
    printf 'Form: sudo ./install.sh [-n]\n'
    printf 'Where "-n" is an optional parameteter for no HTTPS mode\n'
    exit 1
}

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

if [ $# -ne 0 ]; then
    while getopts ":n" flag
    do
        case "${flag}" in 
            n)  NOHTTPS=1;;
            \?) help;;
        esac
    done
fi

#If install.sh in admin/misc, need to change directory
LSINSTALL_DIR=`dirname "$0"`
#cd $LSINSTALL_DIR/

init
install_dir
VERSION=open

INSTALL_TYPE="install"
if [ -f "$LSWS_HOME/lsws-prometheus-exporter" ] ; then
    INSTALL_TYPE="upgrade"
    stopExporter
else
    if [ $NOHTTPS -eq 0 ]; then
        getCerts
    fi
fi

echo "INSTALL_TYPE is $INSTALL_TYPE"
echo "LSINSTALL_DIR:$LSINSTALL_DIR "

installation
if [ $INSTALL_TYPE = "install" ]; then
    $LSWS_HOME/rc-inst.sh
    #"$LSWS_HOME/lsws-prometheus-exporter" $CERT_FILE $KEY_FILE 2>&1 1>/dev/null &
    service lsws-prometheus-exporter start
fi
   
printf "\n\033[38;5;148mInstallation finished, Enjoy!\033[39m\n"


