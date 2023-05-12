# Copyright (C) The Lightning Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

cwlVersion: v1.2
class: Workflow
label: Impute gVCF and convert to FASTA for gVCF with NON_REF
requirements:
  ScatterFeatureRequirement: {}
  SubworkflowFeatureRequirement: {}
hints:
  DockerRequirement:
    dockerPull: vcfutil

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
  chrs:
    type: string[]
  refsdir: Directory
  mapsdir: Directory
  panelnocallbed: File

outputs:
  fas:
    type: File[]
    label: Output pair of FASTAs
    outputSource: bcftools-consensus/fas

steps:
  fixvcf-get_bed_varonlyvcf:
    run: ../helpers/fixvcf-get_bed_varonlyvcf.cwl
    in:
      sampleid: sampleid
      vcf: vcf
      gqcutoff: gqcutoff
      genomebed: genomebed
    out: [nocallbed, varonlyvcf]

  imputation-wf:
    run: ../../../../imputation/imputation-wf.cwl
    in:
      sample: sampleid
      chrs: chrs
      refsdir: refsdir
      mapsdir: mapsdir
      vcf: fixvcf-get_bed_varonlyvcf/varonlyvcf
      nocallbed: fixvcf-get_bed_varonlyvcf/nocallbed
      panelnocallbed: panelnocallbed
      genomebed: genomebed
      panelcallbed: panelnocallbed
    out: [phasedimputedvcf, phasedimputednocallbed]

  bcftools-consensus:
    run: ../helpers/bcftools-consensus.cwl
    in:
      sampleid: sampleid
      vcf: imputation-wf/phasedimputedvcf
      ref: ref
      mask: imputation-wf/phasedimputednocallbed
    out: [fas]
