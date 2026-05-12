/*
 * C tests for libonce with assertions.
 *
 * Compile:
 *   make c-test
 *
 * Run:
 *   ./examples/test
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <pthread.h>
#include "../cgo/libonce.h"

#define ASSERT(cond, msg) do { \
    if (!(cond)) { \
        fprintf(stderr, "FAIL: %s (line %d): %s\n", __func__, __LINE__, msg); \
        exit(1); \
    } \
} while(0)

#define PASS(name) printf("PASS: %s\n", name)

void test_init_destroy(void) {
    once_init();
    once_destroy();
    PASS("test_init_destroy");
}

void test_check_not_found(void) {
    once_init();

    char response[1024];
    int result = once_check("nonexistent-key", response, sizeof(response));
    ASSERT(result == 0, "expected 0 for not found");

    once_destroy();
    PASS("test_check_not_found");
}

void test_store_and_check(void) {
    once_init();

    const char *key = "test-key-1";
    const char *body = "{\"status\":\"ok\"}";

    int result = once_store((char*)key, 200, (char*)body, strlen(body), 3600);
    ASSERT(result == 0, "store should return 0");

    char response[1024];
    int status_code;
    int body_len;

    result = once_check_full((char*)key, &status_code, response, sizeof(response), &body_len);
    ASSERT(result == 1, "check_full should return 1 for found");
    ASSERT(status_code == 200, "status code should be 200");
    ASSERT(body_len == (int)strlen(body), "body length mismatch");
    ASSERT(strcmp(response, body) == 0, "body content mismatch");

    once_destroy();
    PASS("test_store_and_check");
}

void test_lock_unlock(void) {
    once_init();

    const char *key = "lock-key";

    int lock_id = once_lock((char*)key);
    ASSERT(lock_id >= 0, "first lock should succeed");

    int lock_id2 = once_lock((char*)key);
    ASSERT(lock_id2 == -1, "second lock should fail with -1");

    once_unlock(lock_id);

    int lock_id3 = once_lock((char*)key);
    ASSERT(lock_id3 >= 0, "lock after unlock should succeed");
    once_unlock(lock_id3);

    once_destroy();
    PASS("test_lock_unlock");
}

void test_multiple_keys(void) {
    once_init();

    for (int i = 0; i < 100; i++) {
        char key[64];
        char body[128];
        snprintf(key, sizeof(key), "key-%d", i);
        snprintf(body, sizeof(body), "{\"id\":%d}", i);

        int result = once_store(key, 200 + (i % 5), body, strlen(body), 3600);
        ASSERT(result == 0, "store should succeed");
    }

    for (int i = 0; i < 100; i++) {
        char key[64];
        char expected_body[128];
        char response[128];
        int status_code;
        int body_len;

        snprintf(key, sizeof(key), "key-%d", i);
        snprintf(expected_body, sizeof(expected_body), "{\"id\":%d}", i);

        int result = once_check_full(key, &status_code, response, sizeof(response), &body_len);
        ASSERT(result == 1, "check should find key");
        ASSERT(status_code == 200 + (i % 5), "status code mismatch");
        ASSERT(strcmp(response, expected_body) == 0, "body mismatch");
    }

    once_destroy();
    PASS("test_multiple_keys");
}

#define NUM_THREADS 50

typedef struct {
    const char *key;
    int success;
} thread_arg_t;

void *thread_lock(void *arg) {
    thread_arg_t *ta = (thread_arg_t*)arg;

    int lock_id = once_lock((char*)ta->key);
    if (lock_id >= 0) {
        ta->success = 1;
        /* Hold lock briefly */
        usleep(10000);
        once_unlock(lock_id);
    } else {
        ta->success = 0;
    }

    return NULL;
}

void test_thundering_herd(void) {
    once_init();

    pthread_t threads[NUM_THREADS];
    thread_arg_t args[NUM_THREADS];
    const char *key = "thundering-key";

    for (int i = 0; i < NUM_THREADS; i++) {
        args[i].key = key;
        args[i].success = 0;
    }

    for (int i = 0; i < NUM_THREADS; i++) {
        pthread_create(&threads[i], NULL, thread_lock, &args[i]);
    }

    for (int i = 0; i < NUM_THREADS; i++) {
        pthread_join(threads[i], NULL);
    }

    int success_count = 0;
    for (int i = 0; i < NUM_THREADS; i++) {
        success_count += args[i].success;
    }

    /* Only one thread should have acquired the lock at the start */
    ASSERT(success_count >= 1, "at least one thread should succeed");
    printf("  thundering herd: %d/%d threads got lock\n", success_count, NUM_THREADS);

    once_destroy();
    PASS("test_thundering_herd");
}

void test_empty_body(void) {
    once_init();

    const char *key = "empty-body-key";

    int result = once_store((char*)key, 204, NULL, 0, 3600);
    ASSERT(result == 0, "store with empty body should succeed");

    char response[64];
    int status_code;
    int body_len;

    result = once_check_full((char*)key, &status_code, response, sizeof(response), &body_len);
    ASSERT(result == 1, "should find key");
    ASSERT(status_code == 204, "status should be 204");
    ASSERT(body_len == 0, "body should be empty");

    once_destroy();
    PASS("test_empty_body");
}

int main(void) {
    printf("Running C tests for libonce...\n\n");

    test_init_destroy();
    test_check_not_found();
    test_store_and_check();
    test_lock_unlock();
    test_multiple_keys();
    test_thundering_herd();
    test_empty_body();

    printf("\nAll tests passed!\n");
    return 0;
}
