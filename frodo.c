#include "_cgo_export.h"

#define QUEUE_DEPTH 16
#define BLOCK_SZ    1024

struct io_uring ring;

struct file_info {
    off_t file_sz;
    struct iovec iovecs[];
};

int wait() {
    struct io_uring_cqe *cqe;
    int ret = io_uring_wait_cqe(&ring, &cqe);
    if (ret < 0) {
        perror("io_uring_wait_cqe");
        return 1;
    }
    if (cqe->res < 0) {
        fprintf(stderr, "Async readv failed.\n");
        return 1;
    }
    struct file_info *fi = io_uring_cqe_get_data(cqe);
    int blocks = (int) fi->file_sz / BLOCK_SZ;
    if (fi->file_sz % BLOCK_SZ) blocks++;
    for (int i = 0; i < blocks; i ++)
        printToConsole(fi->iovecs[i].iov_base);

    io_uring_cqe_seen(&ring, cqe);
    return 0;
}

int submit_read_request(int file_fd, off_t file_sz) {
    off_t bytes_remaining = file_sz;
    off_t offset = 0;
    int current_block = 0;
    int blocks = (int) file_sz / BLOCK_SZ;
    if (file_sz % BLOCK_SZ) blocks++;
    struct file_info *fi = malloc(sizeof(*fi) + (sizeof(struct iovec) * blocks));
    char *buff = malloc(file_sz);
    if (!buff) {
        fprintf(stderr, "Unable to allocate memory.\n");
        return 1;
    }

    while (bytes_remaining) {
        off_t bytes_to_read = bytes_remaining;
        if (bytes_to_read > BLOCK_SZ)
            bytes_to_read = BLOCK_SZ;

        offset += bytes_to_read;
        fi->iovecs[current_block].iov_len = bytes_to_read;

        void *buf;
        if( posix_memalign(&buf, BLOCK_SZ, BLOCK_SZ)) {
            perror("posix_memalign");
            return 1;
        }
        fi->iovecs[current_block].iov_base = buf;

        current_block++;
        bytes_remaining -= bytes_to_read;
    }
    fi->file_sz = file_sz;

    /* Get an SQE */
    struct io_uring_sqe *sqe = io_uring_get_sqe(&ring);
    /* Setup a readv operation */
    io_uring_prep_readv(sqe, file_fd, fi->iovecs, blocks, 0);
    /* Set user data */
    io_uring_sqe_set_data(sqe, fi);
    /* Finally, submit the request */
    io_uring_submit(&ring);

    return 0;
}

int queue_init() {
    return io_uring_queue_init(QUEUE_DEPTH, &ring, 0);
}

void queue_exit() {
    io_uring_queue_exit(&ring);
}
