#include "test.h"
extern void C2GO();
void test() {
	printf("c test func\n");
	fflush(stdout);
	C2GO();
}
void GO2C() {
	printf("GO2C reaches C side\n");
	fflush(stdout);
}