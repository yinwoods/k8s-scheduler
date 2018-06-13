FROM scratch
MAINTAINER yinwoods <yinwoods@163.com>
ADD scheduler /scheduler
ENTRYPOINT ["/scheduler"]
