// Prints one random input. argv[1] is the seed.
#include <bits/stdc++.h>
using namespace std;

int main(int argc, char** argv) {
    mt19937 rng(atoi(argv[1]));
    cout << rng() % 10 + 1 << "\n";
}
