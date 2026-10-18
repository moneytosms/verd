// Prints one random input. argv[1] is the seed; $SIZE grows from 1 to 10 so early failures are small.
#include <stdio.h>
#include <stdlib.h>

int main(int argc, char **argv) {
    srand(atoi(argv[1]));
    int size = getenv("SIZE") ? atoi(getenv("SIZE")) : 10;
    int n = rand() % size + 1;
    printf("%d\n", n);
    for (int i = 0; i < n; i++) printf("%d%c", rand() % size + 1, i == n - 1 ? '\n' : ' ');
}
