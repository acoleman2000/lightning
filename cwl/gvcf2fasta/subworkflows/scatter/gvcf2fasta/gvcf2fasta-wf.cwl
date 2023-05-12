# Copyright (C) The Lightning Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

cwlVersion: v1.2
class: Workflow
label: Convert gVCF to FASTA
requirements:
  ScatterFeatureRequirement: {}
hints:
  DockerRequirement:
    dockerPull: vcfutil
  ResourceRequirement:
    ramMin: 5000

inputs:
  sampleid:
    type: string
    label: Sample ID
  vcf:
    type: File
    label: Input gVCF
  gqcutoff:
    type: int
    label: GQ (Genotype Quality) cutoff for filtering
  genomebed:
    type: File
    label: Whole genome BED
  ref:
    type: File
    label: Reference FASTA
    
outputs:
  fas:
    type: File[]
    label: Output pair of FASTAs
    outputSource: bcftools-consensus/fas

steps:
  get_bed_varonlyvcf:
    run: ../helpers/get_bed_varonlyvcf.cwl
    in:
      sampleid: sampleid
      vcf: vcf
      gqcutoff: gqcutoff
      genomebed: genomebed
    out: [nocallbed, varonlyvcf]

  bcftools-consensus:
    run: ../helpers/bcftools-consensus.cwl
    scatter: haplotype
    in:
      sampleid: sampleid
      vcf: get_bed_varonlyvcf/varonlyvcf
      ref: ref
      mask: get_bed_varonlyvcf/nocallbed
    out: [fas]
