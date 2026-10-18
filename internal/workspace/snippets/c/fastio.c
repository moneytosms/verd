// fast input: read_int() reads one integer through a 64 KiB buffer (much faster than scanf)
#include <stdio.h>
static char ibuf[1 << 16];
static size_t ipos, ilen;
static int read_char(void) {
    if (ipos == ilen) {
        ilen = fread(ibuf, 1, sizeof ibuf, stdin);
        ipos = 0;
        if (ilen == 0) return -1;
    }
    return ibuf[ipos++];
}
static long long read_int(void) {
    int c = read_char();
    while (c != '-' && (c < '0' || c > '9')) {
        if (c < 0) return 0;
        c = read_char();
    }
    int neg = c == '-';
    if (neg) c = read_char();
    long long x = 0;
    for (; c >= '0' && c <= '9'; c = read_char()) x = x * 10 + (c - '0');
    return neg ? -x : x;
}
