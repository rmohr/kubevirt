FROM fedora

MAINTAINER "Stu Gott" <sgott@redhat.com>
ENV container docker

# util-linux listed here explicity as nsenter must be present in this image
RUN yum install -y \
  libvirt-daemon-kvm \
  libvirt-daemon-qemu \
  libvirt-client \
  selinux-policy selinux-policy-targeted \
  augeas \
  util-linux \
  libcgroup-tools \
  ethtool \
  sudo \
  docker && yum -y clean all

COPY augconf /augconf
RUN augtool -f /augconf

COPY qemu-kube /usr/local/bin/qemu-x86_64
RUN chmod a+x /usr/local/bin/qemu-x86_64

COPY libvirtd.sh /libvirtd.sh
RUN chmod a+x /libvirtd.sh

CMD ["/libvirtd.sh"]
