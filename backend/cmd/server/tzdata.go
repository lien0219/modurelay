package main

// The IANA timezone database is embedded so LoadLocation("Asia/Shanghai")
// works in images that do not ship /usr/share/zoneinfo. Images that already
// install tzdata keep using the system database first.
import _ "time/tzdata"
