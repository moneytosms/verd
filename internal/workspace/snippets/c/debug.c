// debug print to stderr: dbg("%d", x) prints "x = value"
#define dbg(fmt, x) fprintf(stderr, #x " = " fmt "\n", x)
