# CS-733
A server application based on memcache which accepts and responds to multiple clients simultaneously

Following commands are implemented
1.  Set: create the key-value pair, or update the value if it already exists.

    set <key> <exptime> <numbytes> [noreply]\r\n
    <value bytes>\r\n

    The server responds with:

    OK <version>\r\n  

    where version is a unique 64-bit number (in decimal format) assosciated with the key.

2.  Get: Given a key, retrieve the corresponding key-value pair

    get <key>\r\n

    The server responds with the following format

    VALUE <numbytes>\r\n
    <value bytes>\r\n

3.  Get Meta: Retrieve value, version number and expiry time left

     getm <key>\r\n

    The server responds with the following format 

    VALUE <version> <exptime> <numbytes>\r\n
    <value bytes>\r\n

4.  Compare and swap. This replaces the old value (corresponding to key) with the new value only if the version is still the same.

    cas <key> <exptime> <version> <numbytes> [noreply]\r\n
    <value bytes>\r\n

    The server responds with the new version if successful

      OK <version>\r\n

5.  Delete key-value pair

     delete <key>\r\n

    Server response (if successful)

      DELETED\r\n

Errors that can be returned.

    “ERR_VERSION \r\n” (the value was not changed because of a version mismatch)
    “ERRNOTFOUND\r\n” (the key doesn’t exist)
    “ERRCMDERR\r\n” (the command line is not formatted correctly)
    “ERR_INTERNAL\r\n (if command entered is none of the supported ones)



