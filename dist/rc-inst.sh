#!/bin/sh
CURDIR=`dirname "$0"`
cd $CURDIR
CURDIR=`pwd`

INST_USER=`id`
INST_USER=`expr "$INST_USER" : 'uid=.*(\(.*\)) gid=.*'`
if [ $INST_USER != "root" ]; then
	cat <<EOF
[ERROR] Only root user can install the service script!
EOF
	exit 1
fi
INIT_DIR=""



if [ "x`uname -s`" = "xDarwin" ]; then

	STARTUP_ITEM=/System/Library/StartupItems/lsws-prometheus-exporter
	if [ ! -d $STARTUP_ITEM ]; then
		mkdir $STARTUP_ITEM
	fi
	cp "$CURDIR/lsws-prometheus-exporter.rc" $STARTUP_ITEM/lsws-prometheus-exporter
	cat <<EOF >$STARTUP_ITEM/StartupParameters.plist
{
  Description     = "LiteSpeed Web Server Prometheus Exporter";
  Provides        = ("LiteSpeed_Prometheus Services");
  Requires        = ("DirectoryServices");
  Uses            = ("Disks", "NFS");
  OrderPreference = "None";
}

EOF


	exit 0
fi

if [ "x`uname -s`" = "xFreeBSD" ]; then
    echo "Running FreeBSD"
    if [ ! -d "/usr/local/etc/rc.d" ]; then
        if [ ! -d "/usr/local/etc" ]; then
            cat <<EOF
[ERROR] /usr/local/etc/ does not exit in this FreeBSD system!

EOF
            exit 1
        fi
        mkdir /usr/local/etc/rc.d
	fi
    cp "$CURDIR/lsws-prometheus-exporter.rc" /usr/local/etc/rc.d/lsws-prometheus-exporter.sh
    chmod 755 /usr/local/etc/rc.d/lsws-prometheus-exporter.sh
    echo "[OK] The startup script has been successfully installed!"
    exit 0
else
	if [ -f "/etc/gentoo-release" ]; then
		cp "$CURDIR/lsws-prometheus-exporter.rc.gentoo" /etc/init.d/lsws-prometheus-exporter
		chmod a+x /etc/init.d/lsws-prometheus-exporter
		rc-update add lsws-prometheus-exporter default
		exit 0
	fi

    grep "Debian" /etc/issue 2>/dev/null 1>&2
    if [ $? -eq 0  ]; then
        cp "$CURDIR/lsws-prometheus-exporter.rc" /etc/init.d/lsws-prometheus-exporter
        chmod a+x /etc/init.d/lsws-prometheus-exporter
        update-rc.d lsws-prometheus-exporter defaults
        exit 0
    fi
fi 

echo "SystemD system"
for path in /etc/init.d /etc/rc.d/init.d
do
    if [ "x$INIT_DIR" = "x" ]; then
        if [ -d "$path" ]; then
            INIT_DIR=$path
        fi
    fi
done


SYSTEMDDIR=""

SYSTEMBIN=`which systemctl 2>/dev/null`
if [ $? -eq 0 ] ; then
    for path in /etc/systemd/system /usr/lib/systemd/system /lib/systemd/system
    do
        if [ "${SYSTEMDDIR}" = "" ] ; then
            if [ -d "$path" ] ; then
                SYSTEMDDIR=$path
            fi
        fi
    done

    #DirectAdmin may not have /etc/systemd/system/httpd.service, but need to use systemd
    if [ "${SYSTEMDDIR}" = "" ] && [ -d /usr/local/directadmin ] && [ -d /etc/systemd/system ]; then
        SYSTEMDDIR=/etc/systemd/system
    fi

    if [ "${SYSTEMDDIR}" = "" ] && [ -f /etc/redhat-release ] && [ -d /usr/lib/systemd/system ]; then
        SYSTEMDDIR=/usr/lib/systemd/system
    fi
fi

if [ "${SYSTEMDDIR}" != "" ] ; then
    if [ "${INIT_DIR}" != "" ] && [ -e ${INIT_DIR}/lsws-prometheus-exporter ] ; then
        echo "Removing ${INIT_DIR}/lsws-prometheus-exporter"
        rm -f ${INIT_DIR}/lsws-prometheus-exporter
    fi

    cp -f ${CURDIR}/lsws-prometheus-exporter.service ${SYSTEMDDIR}/lsws-prometheus-exporter.service
    chmod 644 ${SYSTEMDDIR}/lsws-prometheus-exporter.service
    #ln -sf ${SYSTEMDDIR}/lsws-prometheus-exporter.service ${SYSTEMDDIR}/lsws-prometheus-exporter.service

    systemctl daemon-reload
    systemctl enable lsws-prometheus-exporter.service
    if [ $? -eq 0  ]; then
            echo "[OK] lsws-prometheus-exporter.service has been successfully installed!"
            exit 0
    else
        echo "[ERROR] failed to enable lsws-prometheus-exporter.service in systemd!"
        exit 1
    fi
fi



if [ "x$INIT_DIR" = "x" ]; then
	echo "[ERROR] failed to find the init.d directory!"
	exit 1
fi

if [ -f "$INIT_DIR/lsws-prometheus-exporter" ]; then
	rm -f $INIT_DIR/lsws-prometheus-exporter
fi

if [ -d "$INIT_DIR/rc2.d" ]; then
        INIT_BASE_DIR=$INIT_DIR
else
        INIT_BASE_DIR=`dirname $INIT_DIR`
fi

cp "$CURDIR/lsws-prometheus-exporter.rc" $INIT_DIR/lsws-prometheus-exporter
chmod 755 $INIT_DIR/lsws-prometheus-exporter


if [ -d "$INIT_BASE_DIR/runlevel/default" ]; then
	ln -fs $INIT_DIR/lsws-prometheus-exporter $INIT_BASE_DIR/runlevel/default/S88lsws-prometheus-exporter
	ln -fs $INIT_DIR/lsws-prometheus-exporter $INIT_BASE_DIR/runlevel/default/K12lsws-prometheus-exporter
fi


if [ -d "$INIT_BASE_DIR/rc2.d" ]; then
	ln -fs $INIT_DIR/lsws-prometheus-exporter $INIT_BASE_DIR/rc2.d/S88lsws-prometheus-exporter
	ln -fs $INIT_DIR/lsws-prometheus-exporter $INIT_BASE_DIR/rc2.d/K12lsws-prometheus-exporter
fi

if [ -d "$INIT_BASE_DIR/rc3.d" ]; then
ln -fs $INIT_DIR/lsws-prometheus-exporter $INIT_BASE_DIR/rc3.d/S88lsws-prometheus-exporter
ln -fs $INIT_DIR/lsws-prometheus-exporter $INIT_BASE_DIR/rc3.d/K12lsws-prometheus-exporter
fi

if [ -d "$INIT_BASE_DIR/rc5.d" ]; then
ln -fs $INIT_DIR/lsws-prometheus-exporter $INIT_BASE_DIR/rc5.d/S88lsws-prometheus-exporter
ln -fs $INIT_DIR/lsws-prometheus-exporter $INIT_BASE_DIR/rc5.d/K12lsws-prometheus-exporter
fi

if [ -d "$INIT_BASE_DIR/rc0.d" ]; then
ln -fs $INIT_DIR/lsws-prometheus-exporter $INIT_BASE_DIR/rc0.d/K12lsws-prometheus-exporter
fi

if [ -d "$INIT_BASE_DIR/rc1.d" ]; then
ln -fs $INIT_DIR/lsws-prometheus-exporter $INIT_BASE_DIR/rc1.d/K12lsws-prometheus-exporter
fi

if [ -d "$INIT_BASE_DIR/rc6.d" ]; then
ln -fs $INIT_DIR/lsws-prometheus-exporter $INIT_BASE_DIR/rc6.d/K12lsws-prometheus-exporter
fi

echo "[OK] The startup script has been successfully installed!"

exit 0
