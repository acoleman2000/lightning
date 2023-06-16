# Copyright (C) The Lightning Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

$namespaces:
  arv: "http://arvados.org/cwl#"
cwlVersion: v1.1
class: Workflow
label: Scatter to Convert gVCF to FASTA
requirements:
  SubworkflowFeatureRequirement: {}
  ScatterFeatureRequirement: {}
hints:
  DockerRequirement:
    dockerPull: vcfutil
  arv:IntermediateOutput:
    outputTTL: 604800

inputs:
  vcfs:
    type: File[]
    label: Input files of VCFs
  sampleids:
    type: string[]
    label: Sample IDs
  genomebed:
    type: File
    label: Whole genome BED
  ref:
    type: File
    label: Reference FASTA
  gqcutoff:
    type: int
    label: GQ (Genotype Quality) cutoff for filtering

outputs:
  fas:
    type:
      type: array
      items:
        type: array
        items: File
    label: Output pairs of FASTAs
    outputSource: gvcf2fasta-wf/fas

steps:
  gvcf2fasta-wf:
    run: gvcf2fasta/gvcf2fasta-wf.cwl
    scatter: [sampleid, vcf]
    scatterMethod: dotproduct
    in:
      sampleid: sampleids
      vcf: vcfs
      genomebed: genomebed
      ref: ref
      gqcutoff: gqcutoff
    out: [fas]
