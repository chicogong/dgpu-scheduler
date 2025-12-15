#!/bin/bash

# Fake nvidia-smi for testing purposes
# Simulates 4 GPUs

if [[ "$1" == "--query-gpu=index,name,memory.total" ]]; then
    cat <<EOF
0, Tesla V100-SXM2-32GB, 32768
1, Tesla V100-SXM2-32GB, 32768
2, Tesla V100-SXM2-32GB, 32768
3, Tesla V100-SXM2-32GB, 32768
EOF
else
    echo "NVIDIA-SMI 550.54.15    Driver Version: 550.54.15    CUDA Version: 12.4"
    echo ""
    cat <<EOF
+-----------------------------------------------------------------------------+
| NVIDIA-SMI 550.54.15    Driver Version: 550.54.15    CUDA Version: 12.4   |
|-------------------------------+----------------------+----------------------+
| GPU  Name        Persistence-M| Bus-Id        Disp.A | Volatile Uncorr. ECC |
|============================================================================================|
|   0  Tesla V100-SXM2     Off  | 00000000:00:04.0 Off |                    0 |
|   1  Tesla V100-SXM2     Off  | 00000000:00:05.0 Off |                    0 |
|   2  Tesla V100-SXM2     Off  | 00000000:00:06.0 Off |                    0 |
|   3  Tesla V100-SXM2     Off  | 00000000:00:07.0 Off |                    0 |
+-----------------------------------------------------------------------------+
EOF
fi
