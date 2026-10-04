/* Prints one random input. argv[1] is the seed. */
#include <stdio.h>
#include <stdlib.h>

int main(int argc, char **argv) {
    srand(atoi(argv[1]));
    printf("%d\n", rand() % 10 + 1);
    return 0;
}
