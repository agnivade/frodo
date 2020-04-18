#include "_cgo_export.h"

#define QUEUE_DEPTH 16
#define BLOCK_SZ    1024

struct io_uring ring;

struct file_info {
    __u8 opcode;
    off_t file_sz;
    int file_fd;
    struct iovec iovecs[];
};

int pop_request() {
    struct io_uring_cqe *cqe;
    // Get from queue.
    int ret = io_uring_peek_cqe(&ring, &cqe);
    if (ret < 0) {
        fprintf(stderr, "bad ret.\n");
        return ret;
    }
    if (cqe->res < 0) {
        fprintf(stderr, "bad res.\n");
        return cqe->res;
    }

    struct file_info *fi = io_uring_cqe_get_data(cqe);
    if (fi->opcode == IORING_OP_READV) {
        int blocks = (int) fi->file_sz / BLOCK_SZ;
        if (fi->file_sz % BLOCK_SZ) blocks++;
        // Call read_callback to Go.
        read_callback(fi->iovecs, blocks, fi->file_fd);
    } else if (fi->opcode == IORING_OP_WRITEV) {
        // Call write_callback to Go.
        write_callback(cqe->res, fi->file_fd);
    }

    // Mark as done.
    io_uring_cqe_seen(&ring, cqe);
    return 0;
}

int push_read_request(int file_fd, off_t file_sz) {
    off_t bytes_remaining = file_sz;
    off_t offset = 0;
    int current_block = 0;
    int blocks = (int) file_sz / BLOCK_SZ;
    if (file_sz % BLOCK_SZ) blocks++;
    struct file_info *fi = malloc(sizeof(*fi) + (sizeof(struct iovec) * blocks));
    if (!fi) {
        fprintf(stderr, "Unable to allocate memory\n");
        return -1;
    }
    // char *buf = malloc(file_sz);
    // if (!buf) {
    //     fprintf(stderr, "Unable to allocate memory.\n");
    //     return -1;
    // }

    // Populate iovecs.
    while (bytes_remaining) {
        off_t bytes_to_read = bytes_remaining;
        if (bytes_to_read > BLOCK_SZ)
            bytes_to_read = BLOCK_SZ;

        offset += bytes_to_read;
        fi->iovecs[current_block].iov_len = bytes_to_read;

        void *buf;
        if( posix_memalign(&buf, BLOCK_SZ, BLOCK_SZ)) {
            perror("posix_memalign");
            return -1;
        }
        fi->iovecs[current_block].iov_base = buf;

        current_block++;
        bytes_remaining -= bytes_to_read;
    }
    fi->file_sz = file_sz;
    fi->file_fd = file_fd;
    fi->opcode = IORING_OP_READV;

    // Set the queue.
    struct io_uring_sqe *sqe = io_uring_get_sqe(&ring);
    io_uring_prep_readv(sqe, file_fd, fi->iovecs, blocks, 0);
    io_uring_sqe_set_data(sqe, fi);
    return 0;
}

int push_write_request(int file_fd, void *data, off_t file_sz) {
    off_t bytes_remaining = file_sz;
    off_t offset = 0;
    struct file_info *fi = malloc(sizeof(*fi) + (sizeof(struct iovec) * 1));

    // TODO: split into multiple blocks.
    fi->iovecs[0].iov_base = data;
    fi->iovecs[0].iov_len = file_sz;
    fi->file_sz = file_sz;
    fi->file_fd = file_fd;
    fi->opcode = IORING_OP_WRITEV;

    // Set the queue.
    struct io_uring_sqe *sqe = io_uring_get_sqe(&ring);
    io_uring_prep_writev(sqe, file_fd, fi->iovecs, 1, 0);
    io_uring_sqe_set_data(sqe, fi);
    return 0;
}

int queue_submit(int num) {
    return io_uring_submit_and_wait(&ring, num);
}

int queue_init() {
    return io_uring_queue_init(QUEUE_DEPTH, &ring, 0);
}

void queue_exit() {
    io_uring_queue_exit(&ring);
}
