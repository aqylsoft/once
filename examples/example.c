/*
 * Example usage of libonce from C.
 *
 * Compile:
 *   make c-example
 *
 * Run:
 *   ./examples/example
 */

#include <stdio.h>
#include <string.h>
#include "../cgo/libonce.h"

int main() {
    char response[4096];
    int status_code;
    int body_len;
    int lock_id;
    int result;

    /* Initialize the store */
    once_init();
    printf("Store initialized\n");

    /* Try to check a non-existent key */
    result = once_check("payment-123", response, sizeof(response));
    printf("Check 'payment-123': %d (0=not found)\n", result);

    /* Acquire a lock */
    lock_id = once_lock("payment-123");
    printf("Lock acquired, id=%d\n", lock_id);

    /* Try to lock again (should fail) */
    result = once_lock("payment-123");
    printf("Second lock attempt: %d (-1=locked)\n", result);

    /* Store a response */
    const char *body = "{\"status\":\"success\",\"payment_id\":\"PAY-123\"}";
    result = once_store("payment-123", 200, body, strlen(body), 3600);
    printf("Store result: %d (0=success)\n", result);

    /* Release the lock */
    once_unlock(lock_id);
    printf("Lock released\n");

    /* Check the key again */
    result = once_check_full("payment-123", &status_code, response, sizeof(response), &body_len);
    printf("Check result: %d, status=%d, body_len=%d\n", result, status_code, body_len);
    printf("Response: %s\n", response);

    /* Cleanup */
    once_destroy();
    printf("Store destroyed\n");

    return 0;
}
