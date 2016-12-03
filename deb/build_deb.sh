sudo apt-get install runit -y

cd ..

chmod 775 deb/pkg-build/DEBIAN/postinst

cd deb

dpkg -b pkg-build

mv pkg-build.deb tz-mcall.deb
