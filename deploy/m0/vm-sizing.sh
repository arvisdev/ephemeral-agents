# VM sizing for the ephemeral-agents POC estate — single source of truth.
# Referenced by: M0 install doc section 1 (qm create), M3/M4 install docs (sizing notes).
#
# Two profiles. The `qm create` lines in the M0 doc use the MINIMUM values.
# If real executor images (M3/M5) or >=2 concurrent executors will run, create the
# VMs with the RECOMMENDED values instead (or resize vm-xb/vm-hindsight before M3).
# RabbitMQ memory alarms under pressure are the known failure mode (M0 troubleshooting).

# ---- MINIMUM (POC baseline; fine through the M0-M2 gates with the stub executor) ----
VM_XA_MIN_MEM_MB=4096        ; VM_XA_MIN_CORES=2 ; VM_XA_MIN_DISK_GB=20
VM_XB_MIN_MEM_MB=8192        ; VM_XB_MIN_CORES=4 ; VM_XB_MIN_DISK_GB=40
VM_HINDSIGHT_MIN_MEM_MB=4096 ; VM_HINDSIGHT_MIN_CORES=2 ; VM_HINDSIGHT_MIN_DISK_GB=20

# ---- RECOMMENDED (real executors, concurrency up to the cap of 5) ----
# vm-xb counts ~2 GB RAM per concurrent executor on top of the chassis; disk also
# holds the pre-pulled pinned images (no-pull rule). vm-hindsight: pgvector wants
# RAM for the index; 40 GB disk for a multi-day POC with embeddings.
VM_XA_REC_MEM_MB=4096        ; VM_XA_REC_CORES=2 ; VM_XA_REC_DISK_GB=20
VM_XB_REC_MEM_MB=16384       ; VM_XB_REC_CORES=8 ; VM_XB_REC_DISK_GB=80
VM_HINDSIGHT_REC_MEM_MB=8192 ; VM_HINDSIGHT_REC_CORES=4 ; VM_HINDSIGHT_REC_DISK_GB=40

# Profile selection (source this file, then use e.g. ${VM_XB_MEM_MB}):
SIZING_PROFILE="${SIZING_PROFILE:-minimum}"   # minimum | recommended
case "$SIZING_PROFILE" in
  minimum)     VM_XA_MEM_MB=$VM_XA_MIN_MEM_MB;             VM_XA_CORES=$VM_XA_MIN_CORES;             VM_XA_DISK_GB=$VM_XA_MIN_DISK_GB
               VM_XB_MEM_MB=$VM_XB_MIN_MEM_MB;             VM_XB_CORES=$VM_XB_MIN_CORES;             VM_XB_DISK_GB=$VM_XB_MIN_DISK_GB
               VM_HINDSIGHT_MEM_MB=$VM_HINDSIGHT_MIN_MEM_MB; VM_HINDSIGHT_CORES=$VM_HINDSIGHT_MIN_CORES; VM_HINDSIGHT_DISK_GB=$VM_HINDSIGHT_MIN_DISK_GB ;;
  recommended) VM_XA_MEM_MB=$VM_XA_REC_MEM_MB;             VM_XA_CORES=$VM_XA_REC_CORES;             VM_XA_DISK_GB=$VM_XA_REC_DISK_GB
               VM_XB_MEM_MB=$VM_XB_REC_MEM_MB;             VM_XB_CORES=$VM_XB_REC_CORES;             VM_XB_DISK_GB=$VM_XB_REC_DISK_GB
               VM_HINDSIGHT_MEM_MB=$VM_HINDSIGHT_REC_MEM_MB; VM_HINDSIGHT_CORES=$VM_HINDSIGHT_REC_CORES; VM_HINDSIGHT_DISK_GB=$VM_HINDSIGHT_REC_DISK_GB ;;
  *) echo "SIZING_PROFILE must be 'minimum' or 'recommended'" >&2; exit 1 ;;
esac
