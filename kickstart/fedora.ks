text --non-interactive
eula --agreed

keyboard us
lang en_US.UTF-8

network --noipv6 --onboot=yes --bootproto=dhcp --activate

rootpw --lock
firewall --enabled --ssh
selinux --enforcing

bootloader --location=mbr
zerombr
clearpart --all --initlabel
part /boot     --fstype xfs  --size=1024 --label=BOOTFS
part /boot/efi --fstype vfat --size=1024 --label=EFIFS

part pv.01     --size=100    --grow

volgroup sysvg --pesize=4096 pv.01

logvol /              --fstype xfs --name=lv_root  --vgname=sysvg --size=30720 --label=ROOTFS
logvol /home          --fstype xfs --name=lv_home  --vgname=sysvg --size=13312 --label=HOMEFS   --fsoptions="nodev,nosuid"
logvol /tmp           --fstype xfs --name=lv_tmp   --vgname=sysvg --size=5120  --label=TMPFS    --fsoptions="nodev,noexec,nosuid"
logvol /var           --fstype xfs --name=lv_var   --vgname=sysvg --size=15360 --label=VARFS    --fsoptions="nodev"
logvol /var/lib       --fstype xfs --name=lv_lib   --vgname=sysvg --size=51200 --label=VARLIBFS --fsoptions="nodev"
logvol /var/log       --fstype xfs --name=lv_log   --vgname=sysvg --size=5120  --label=LOGFS    --fsoptions="nodev,noexec,nosuid"
logvol /var/log/audit --fstype xfs --name=lv_audit --vgname=sysvg --size=5120  --label=AUDITFS  --fsoptions="nodev,noexec,nosuid"

services --enabled=NetworkManager,sshd

ostreecontainer --url ghcr.io/a1994sc/bootc-images/alma:latest

user --name=sysadmin --plaintext --password=changeit --groups=wheel
# a1994sc public ssh keys
sshkey --username=sysadmin "ecdsa-sha2-nistp521 AAAAE2VjZHNhLXNoYTItbmlzdHA1MjEAAAAIbmlzdHA1MjEAAACFBACa5MIyu4mLLLc0D5Y0eOWV1JnvvSo68pDJAh4SyC1WyMVK1eOIlpyDlfFNu7wev8fPELJEwbT+pCsjH2FVU8qRNAH17nW1EBn9xWOX7rEnpxOp6X485+jeA0t/a2jB6e7Bcn86Xwa1tPEbIKS6eo530KMLagaCFpl9arv1SGWeh6/YAw=="

reboot --eject
