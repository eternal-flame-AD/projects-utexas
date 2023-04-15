#!/usr/bin/env fish
#SBATCH -J hw2_pi          # Job name
#SBATCH -p nvdimm
#SBATCH -o hw2_pi.o%j      # Name of stdout output file
#SBATCH -e hw2_pi.e%j      # Name of stderr error file
#SBATCH -N 3
#SBATCH -n 3
#SBATCH --ntasks-per-node 1
#SBATCH -t 00:30:00        # Run time (hh:mm:ss)
#SBATCH --mail-type=all    # Send email at begin and end of job

echo "Slurm JOBID="$SLURM_JOBID "MEMBIND="$SLURM_MEM_BIND_TYPE

srun --label hostname

module load intel
module load gnuparallel

set -l numa_bound (numactl -s | \
    awk 'BEGIN {n_numa=255} /^(cpu|mem)bind/ { if ($(NF)<n_numa) n_numa=$(NF) } END{print n_numa}')

echo "Parallel over" (math $numa_bound + 1) "NUMA nodes"

function line_shift
    sed "p;" | awk 'NR==1{store=$0;next}1;END{print store}'
end

function srun_trybind -V output -V numa_bound
    set -q output
        or set -l output output
    
    mkdir -p out/$SLURM_JOBID/$output

    srun \
                -n1 -o out/$SLURM_JOBID/$output/none.out \
        parallel --lb -n0 -q numactl $argv ":::" (seq 0 $numa_bound) : \
                -n1 -o out/$SLURM_JOBID/$output/local.out \
        parallel --lb -q numactl -N{} -m{} $argv ":::" (seq 0 $numa_bound) : \
                -n1 -o out/$SLURM_JOBID/$output/shift.out \
        parallel -N2 --lb -q numactl -N{1} -m{2} $argv ":::" (seq 0 $numa_bound | line_shift)    
end

function slurm_seconds -a fmt
    printf "0 %s+p" (echo $fmt | string replace "-" " 24*+" | string replace -a ":" " 60*+") | dc
end

set fish_trace on

lstopo binding_$SLURM_JOB_PARTITION.xml

set -l max_n (awk '/MemFree/ { printf "%d \n", $2 / (40000000/1000000000) }' /proc/meminfo)
echo "Running for upto N="$max_n

set -l cur_n 1000

while [ (math \
    (slurm_seconds (squeue -h -j $SLURM_JOBID -o "%L")) "/" \
    (slurm_seconds (squeue -h -j $SLURM_JOBID -o "%l"))
) -gt .90 -a $cur_n -lt $max_n ]

    echo "Running with N=" $cur_n

    icpc -std=c++17 -o pi.$cur_n.exec -DPIMC_N=$cur_n -xcore-avx512 -O2 pi.cxx
        or break

    output=$cur_n srun_trybind \
        pi.$cur_n.exec
        or break

    set cur_n (math $cur_n "*" 10)
end

