
#!/bin/sh

init()
{
    LSINSTALL_DIR=`pwd`

    export LSINSTALL_DIR

    DIR_MOD=755
    SDIR_MOD=700
    EXEC_MOD=555
    CONF_MOD=600
    DOC_MOD=644

    INST_USER=`id`
    INST_USER=`expr "$INST_USER" : 'uid=.*(\(.*\)) gid=.*'`

    SYS_NAME=`uname -s`
    if [ "x$SYS_NAME" = "xFreeBSD" ] || [ "x$SYS_NAME" = "xNetBSD" ] || [ "x$SYS_NAME" = "xDarwin" ] ; then
        PS_CMD="ps -ax"
        ID_GROUPS="id"
        TEST_BIN="/bin/test"
        ROOTGROUP="wheel"
    else
        PS_CMD="ps -ef"
        ID_GROUPS="id -a"
        TEST_BIN="/usr/bin/test"
        ROOTGROUP="root"
    fi
    SETUP_PHP=0
    SET_LOGIN=0
    ADMIN_PORT=7080
    INSTALL_TYPE="upgrade"
    SERVER_NAME=`uname -n`
    ADMIN_EMAIL="root@localhost"
    AP_PORT_OFFSET=2000
    PHP_SUEXEC=2
    HOST_PANEL=""
    CERT_FILE=""
    KEY_FILE=""

    WS_USER=root
    WS_GROUP=$ROOTGROUP

    DIR_OWN=$WS_USER:$WS_GROUP
    CONF_OWN=$WS_USER:$WS_GROUP
    
}

# Get destination directory
install_dir()
{

    SUCC=0
    INSTALL_TYPE="reinstall"
    SET_LOGIN=1

    if [ $INST_USER = "root" ]; then
        DEST_RECOM="/usr/local/lsws-prometheus-exporter"
        WS_USER="root"
        LSWS_HOME=$DEST_RECOM
    else
        cat <<EOF

[ERROR] You must be the root user to install the LiteSpeed Web Server Prometheus Exporter

EOF
        SUCC=0
        exit 1
    fi

    if [ ! -d /tmp/lshttpd ]; then
        cat <<EOF

[ERROR] You install the exporter on the system running the LiteSpeed Web Server

EOF
        SUCC=0
        exit 1
    fi
    
    if [ ! -d "$LSWS_HOME" ]; then
        mkdir "$LSWS_HOME"
    fi
    export LSWS_HOME

}


getRunningProcess()
{
    RUNNING_PROCESS=`$PS_CMD | grep "${LSWS_HOME}/lsws-prometheus-exporter" | grep -v grep | awk '{ print $2 }'`
}


stopExporter()
{
    getRunningProcess
    if [ "x$RUNNING_PROCESS" != "x" ]; then
        cat <<EOF

LiteSpeed Web Server Prometheus Exporter is running, in order to continue 
installation, the Exporter will be stopped.

EOF
        kill $RUNNING_PROCESS
        getRunningProcess
        if [ "x$RUNNING_PROCESS" != "x" ]; then
            echo "Failed to stop Exporter, abort installation!"
            exit 1
        fi
    fi
}


getCerts()
{
    SUCC=0
    while [ $SUCC -eq "0" ];  do
        printf "Cert file name [ENTER for no HTTPS]: "
        read TMP_CERT
        if [ "x$TMP_CERT" = "x" ]; then
            echo "HTTPS will not be required"
            SUCC=1
        elif [ ! -f "$TMP_CERT" ]; then
            echo "You must specify a file that exists"
        else
            SUCC=0
            while [ $SUCC -eq "0" ]; do
                printf "Key file name: "
                read TMP_KEY
                if [ ! -f "$TMP_KEY" ]; then
                    echo "You must specify a file that exists"
                else
                    SUCC=1
                    CERT_FILE="--tls-cert-file=$TMP_CERT"
                    KEY_FILE="--tls-key-file=$TMP_KEY"
                fi
            done
        fi
    done
}


getBasicAuth()
{
    SUCC=0
    while [ $SUCC -eq "0" ]; do
        printf "User name for basic auth [ENTER for no basic auth]: "
        read TMP_USER
        if [ "x$TMP_USER" = "x" ]; then
            echo "Basic auth will not be used"
            SUCC=1
        else
            SUCC=0
            while [ $SUCC -eq "0" ]; do
                printf "Password file name (must contain plain-text password): "
                read TMP_PASS
                if [ ! -f "$TMP_PASS" ]; then
                    echo "You must specify a fully qualified existing file name"
                else
                    # Refuse password files readable by group or other when
                    # we are running as root. Even a 0644 file in /etc is
                    # accessible to every local user; this is the most
                    # common installation footgun.
                    PERM=$(stat -c '%a' "$TMP_PASS" 2>/dev/null || stat -f '%A' "$TMP_PASS" 2>/dev/null)
                    if [ "x$PERM" != "x" ]; then
                        case "$PERM" in
                            *[1-7][0-7]|*[0-7][1-7])
                                echo "WARNING: $TMP_PASS has overly permissive mode $PERM."
                                echo "         Recommend: chmod 0600 $TMP_PASS"
                                ;;
                        esac
                    fi
                    SUCC=1
                    USER_NAME="--username=$TMP_USER"
                    PASSWORD_FILE="--password-file=$TMP_PASS"
                fi
            done
        fi
    done
}


util_mkdir()
{
    OWNER=$1
    PERM=$2
    shift
    shift
    for arg
      do
      if [ ! -d "$LSWS_HOME/$arg" ]; then
          mkdir "$LSWS_HOME/$arg"
      fi
      chown "$OWNER" "$LSWS_HOME/$arg"
      chmod $PERM  "$LSWS_HOME/$arg"
    done

}


util_cpfile()
{
    OWNER=$1
    PERM=$2
    shift
    shift
    for arg
      do
      if [ -f "$LSINSTALL_DIR/$arg" ]; then
        cp -f "$LSINSTALL_DIR/$arg" "$LSWS_HOME/$arg"
        chown "$OWNER" "$LSWS_HOME/$arg"
        chmod $PERM  "$LSWS_HOME/$arg"
      fi
    done

}

util_ccpfile()
{
    OWNER=$1
    PERM=$2
    shift
    shift
    for arg
      do
      if [ ! -f "$LSWS_HOME/$arg" ] && [ -f "$LSINSTALL_DIR/$arg" ]; then
        cp "$LSINSTALL_DIR/$arg" "$LSWS_HOME/$arg"
      fi
      if [ -f "$LSWS_HOME/$arg" ]; then
        chown "$OWNER" "$LSWS_HOME/$arg"
        chmod $PERM  "$LSWS_HOME/$arg"
      fi
    done
}


util_cpdir()
{
    OWNER=$1
    PERM=$2
    shift
    shift
    for arg
      do
      cp -R "$LSINSTALL_DIR/$arg/"* "$LSWS_HOME/$arg/"
      chown -R "$OWNER" "$LSWS_HOME/$arg/"*
      #chmod -R $PERM  $LSWS_HOME/$arg/*
    done
}

util_cpdirv()
{
    OWNER=$1
    PERM=$2
    shift
    shift
    VERSION=$1
    shift
    for arg
      do
      cp -R "$LSINSTALL_DIR/$arg/"* "$LSWS_HOME/$arg.$VERSION/"
      chown -R "$OWNER" "$LSWS_HOME/$arg.$VERSION"
      $TEST_BIN -L "$LSWS_HOME/$arg"
      if [ $? -eq 0 ]; then
          rm -f "$LSWS_HOME/$arg"
      fi
      FILENAME=`basename $arg`
      ln -sf "./$FILENAME.$VERSION/" "$LSWS_HOME/$arg"
              #chmod -R $PERM  $LSWS_HOME/$arg/*
    done
}

util_cpfilev()
{
    OWNER=$1
    PERM=$2
    shift
    shift
    VERSION=$1
    shift
    for arg
      do
      cp -f "$LSINSTALL_DIR/$arg" "$LSWS_HOME/$arg.$VERSION"
      chown "$OWNER" "$LSWS_HOME/$arg.$VERSION"
      chmod $PERM  "$LSWS_HOME/$arg.$VERSION"
      $TEST_BIN -L "$LSWS_HOME/$arg"
      if [ $? -eq 0 ]; then
          rm -f "$LSWS_HOME/$arg"
      fi
      FILENAME=`basename $arg`
      ln -sf "./$FILENAME.$VERSION" "$LSWS_HOME/$arg"
    done
}


util_cpdir()
{
    OWNER=$1
    PERM=$2
    shift
    shift
    for arg
      do
      cp -R "$LSINSTALL_DIR/$arg/"* "$LSWS_HOME/$arg/"
      chown -R "$OWNER" "$LSWS_HOME/$arg/"*
      #chmod -R $PERM  $LSWS_HOME/$arg/*
    done
}



util_cpdirv()
{
    OWNER=$1
    PERM=$2
    shift
    shift
    VERSION=$1
    shift
    for arg
      do
      if [ -d "$LSINSTALL_DIR/$arg" ]; then
        cp -R "$LSINSTALL_DIR/$arg/"* "$LSWS_HOME/$arg.$VERSION/"
        chown -R "$OWNER" "$LSWS_HOME/$arg.$VERSION"
        $TEST_BIN -L "$LSWS_HOME/$arg"
        if [ $? -eq 0 ]; then
          rm -f "$LSWS_HOME/$arg"
        fi
        FILENAME=`basename $arg`
        ln -sf "./$FILENAME.$VERSION/" "$LSWS_HOME/$arg"
              #chmod -R $PERM  $LSWS_HOME/$arg/*
      fi
    done
}

util_cpfilev()
{
    OWNER=$1
    PERM=$2
    shift
    shift
    VERSION=$1
    shift
    for arg
      do
      if [ -f "$LSINSTALL_DIR/$arg" ]; then

        cp -f "$LSINSTALL_DIR/$arg" "$LSWS_HOME/$arg.$VERSION"
        chown "$OWNER" "$LSWS_HOME/$arg.$VERSION"
        chmod $PERM  "$LSWS_HOME/$arg.$VERSION"
        $TEST_BIN -L "$LSWS_HOME/$arg"
        if [ $? -eq 0 ]; then
            rm -f "$LSWS_HOME/$arg"
        fi
        FILENAME=`basename $arg`
        ln -sf "./$FILENAME.$VERSION" "$LSWS_HOME/$arg"
      fi
    done
}

installation()
{   
    umask 022
    if [ $INST_USER = "root" ]; then
        export PATH=/sbin:/usr/sbin:$PATH
        SDIR_OWN="root:$ROOTGROUP"
        LOGDIR_OWN="root:$WS_GROUP"
        chown $SDIR_OWN $LSWS_HOME
    else
        SDIR_OWN=$DIR_OWN
        LOGDIR_OWN=$DIR_OWN
    fi
    SED_EXPR="s~%CERT_FILE%~$CERT_FILE~;s~%KEY_FILE%~$KEY_FILE~;s~%USER_NAME%~$USER_NAME~;s~%PASSWORD_FILE%~$PASSWORD_FILE~"
    sed "$SED_EXPR" "$LSINSTALL_DIR/lsws-prometheus-exporter.rc.in" > "$LSWS_HOME/lsws-prometheus-exporter.rc"
    sed "$SED_EXPR" "$LSINSTALL_DIR/lsws-prometheus-exporter.rc.gentoo.in" > "$LSWS_HOME/lsws-prometheus-exporter.rc.gentoo"
    sed "$SED_EXPR" "$LSINSTALL_DIR/lsws-prometheus-exporter.service.in" > "$LSWS_HOME/lsws-prometheus-exporter.service"
    cp $LSINSTALL_DIR/lsws-prometheus-exporter $LSWS_HOME/
    cp $LSINSTALL_DIR/install.sh  $LSWS_HOME/
    cp $LSINSTALL_DIR/functions.sh  $LSWS_HOME/
    cp $LSINSTALL_DIR/rc-inst.sh $LSWS_HOME/
    cp $LSINSTALL_DIR/rc-uninst.sh $LSWS_HOME/
    cp $LSINSTALL_DIR/uninstall.sh  $LSWS_HOME/
}


