// Runtime assembly for the EE: MIPS N32 (64-bit GPRs, 32-bit pointers,
// single-float FPU, 16-byte stack alignment). These are the N32 versions of
// TinyGo's src/runtime/asm_mipsx.S and src/internal/task/task_stack_mipsx.S
// (which are o32: sw/lw of 32-bit registers, 8-byte aligned frames). The
// task-switch part must match calleeSavedRegs in TinyGo's
// src/internal/task/task_stack_ps2.go.

// Do not reorder instructions to insert a branch delay slot.
// We know what we're doing, and will manually fill the branch delay slot.
.set noreorder

.section .text.tinygo_scanCurrentStack
.global  tinygo_scanCurrentStack
.type    tinygo_scanCurrentStack, %function
tinygo_scanCurrentStack:
    // Push the callee-saved registers so the conservative GC sees pointers
    // held in them. 96 bytes keeps sp 16-byte aligned.
    addiu $sp, $sp, -96
    sd    $ra, 80($sp)
    sd    $fp, 72($sp)
    sd    $s7, 64($sp)
    sd    $s6, 56($sp)
    sd    $s5, 48($sp)
    sd    $s4, 40($sp)
    sd    $s3, 32($sp)
    sd    $s2, 24($sp)
    sd    $s1, 16($sp)
    sd    $s0,  8($sp)

    // Scan the stack.
    jal   tinygo_scanstack
    move  $a0, $sp // in the branch delay slot

    ld    $ra, 80($sp)
    addiu $sp, $sp, 96
    jr    $ra
    nop

.section .text.tinygo_longjmp
.global tinygo_longjmp
.type   tinygo_longjmp, %function
tinygo_longjmp:
    // Note: the code we jump to assumes a0 is non-zero, which is already the
    // case because that's the defer frame pointer.
    lw $sp, 0($a0) // jumpSP
    lw $a1, 4($a0) // jumpPC
    jr $a1
    nop

// ---------------------------------------------------------------------------
// Goroutine (tasks scheduler) support.
//
//   - callee-saved GPRs s0-s7, fp, ra are 64 bits wide: sd/ld, 8-byte slots;
//   - frame is 128 bytes so sp stays 16-byte aligned;
//   - f20-f31 (single precision) are callee-saved: LLVM's N32 single-float
//     list has the even ones, gcc's all twelve; save all twelve;
//   - gp is not touched: the program is non-PIC and gp is a constant set by
//     crt0, so leaving it alone preserves it (saving it would load 0 from
//     the initial frame of a new goroutine).
// ---------------------------------------------------------------------------

// tinygo_task_exit only exists with a scheduler; weak so that a
// -scheduler=none build (which never runs this code) still links.
.weak tinygo_task_exit

.section .text.tinygo_startTask
.global  tinygo_startTask
.type    tinygo_startTask, %function
tinygo_startTask:
    // Runs on the new stack with the callee-saved registers just popped:
    // s0 = function to call (goroutine start wrapper), s1 = its argument.
    move  $a0, $s1
    jalr  $s0
    nop

    // The goroutine returned: exit it. Tail call.
    j     tinygo_task_exit
    nop

.section .text.tinygo_swapTask
.global  tinygo_swapTask
.type    tinygo_swapTask, %function
tinygo_swapTask:
    // a0 = newStack uintptr
    // a1 = oldStack *uintptr
    addiu $sp, $sp, -128
    sd    $s0,   0($sp)
    sd    $s1,   8($sp)
    sd    $s2,  16($sp)
    sd    $s3,  24($sp)
    sd    $s4,  32($sp)
    sd    $s5,  40($sp)
    sd    $s6,  48($sp)
    sd    $s7,  56($sp)
    sd    $fp,  64($sp)
    sd    $ra,  72($sp)
    swc1  $f20, 80($sp)
    swc1  $f21, 84($sp)
    swc1  $f22, 88($sp)
    swc1  $f23, 92($sp)
    swc1  $f24, 96($sp)
    swc1  $f25, 100($sp)
    swc1  $f26, 104($sp)
    swc1  $f27, 108($sp)
    swc1  $f28, 112($sp)
    swc1  $f29, 116($sp)
    swc1  $f30, 120($sp)
    swc1  $f31, 124($sp)

    // Save the current stack pointer in *oldStack (32-bit pointer).
    sw    $sp, 0($a1)

    // Switch to the new stack.
    move  $sp, $a0

    ld    $s0,   0($sp)
    ld    $s1,   8($sp)
    ld    $s2,  16($sp)
    ld    $s3,  24($sp)
    ld    $s4,  32($sp)
    ld    $s5,  40($sp)
    ld    $s6,  48($sp)
    ld    $s7,  56($sp)
    ld    $fp,  64($sp)
    ld    $ra,  72($sp)
    lwc1  $f20, 80($sp)
    lwc1  $f21, 84($sp)
    lwc1  $f22, 88($sp)
    lwc1  $f23, 92($sp)
    lwc1  $f24, 96($sp)
    lwc1  $f25, 100($sp)
    lwc1  $f26, 104($sp)
    lwc1  $f27, 108($sp)
    lwc1  $f28, 112($sp)
    lwc1  $f29, 116($sp)
    lwc1  $f30, 120($sp)
    lwc1  $f31, 124($sp)
    addiu $sp, $sp, 128

    // Return into the task (tinygo_startTask on the first switch).
    jr    $ra
    nop
