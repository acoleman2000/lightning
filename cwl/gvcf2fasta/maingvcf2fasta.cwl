# Copyright (C) The Lightning Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

$namespaces:
  arv: "http://arvados.org/cwl#"
cwlVersion: v1.2
class: Workflow
label: Scatter to Convert various gVCF to FASTA
requirements:
  SubworkflowFeatureRequirement: {}
  ScatterFeatureRequirement: {}
  MultipleInputFeatureRequirement: {}
  InlineJavascriptRequirement: {}
  StepInputExpressionRequirement: {}
hints:
  DockerRequirement:
    dockerPull: vcfutil
  arv:IntermediateOutput:
    outputTTL: 604800
  
inputs:
  splitvcfdirs:
    type: Directory[]?
    label: Input directory of split gVCFs
    default: null
  vcfsdir:
    type: Directory?
    label: Input directory of VCFs
    default: null
  vcfs:
    type: File[]?
    label: Input VCFs in array of files 
    default: null
  genomebed:
    type: File?
    label: Whole genome BED
    default: null
  ref:
    type: File?
    label: Reference FASTA
    default: null
  gqcutoff:
    type: int?
    label: GQ (Genotype Quality) cutoff for filtering
    default: null
  sampleids:
    type: string[]?
    label: Sample IDs
    default: null
  chrs: string[]?
  refsdir: Directory?
  mapsdir: Directory?
  panelnocallbed: File?
  panelcallbed: File?
  nonref: boolean?
  split: boolean?
  tar: boolean?


outputs:
  fas:
    type:
      type: array
      items:
        type: array
        items: File
    label: Output pairs of FASTAs
    outputSource: 
      - gvcf2fasta_nonrefvcf-wf/fas
      - gvcf2fasta_splitvcf-imputation-wf/fas
      - gvcf2fasta_splitvcf-wf/fas
      - gvcf2fasta_splitvcftar-wf/fas
      - gvcf2fasta-wf/fas
    pickValue: first_non_null

steps: 
  getfiles:
    run: subworkflows/scatter/helpers/getfiles.cwl
    when: $(inputs.dir !== null)
    in:
      dir: vcfsdir
    out: [vcfs]

  vcf_throttle:
    in:
      vcf_files: 
        source: vcfs
        default: null
      transformed_vcfs: 
        source: getfiles/vcfs
        default: null
    run: subworkflows/scatter/helpers/get_vcfs.cwl
    out: [vcfs]

  get_sample_ids:
    run: subworkflows/scatter/helpers/get_sample_ids.cwl
    when: $(inputs.sampleids === null)
    in:
      vcfs: vcf_throttle/vcfs
      sampleids: sampleids
    out: [sampleids]

  gvcf2fasta_nonrefvcf-wf:
    run:  subworkflows/scatter/gvcf2fasta/gvcf2fasta_nonrefvcf-wf.cwl
    when: $(inputs.vcf !== null && inputs.genomebed !== null && inputs.ref !== null && inputs.gqcutoff !== null && inputs.nonref === true)
    scatter: [sampleid, vcf]
    scatterMethod: dotproduct
    in:
      sampleid: 
        source: get_sample_ids/sampleids
        default: []
      vcf: 
        source: vcf_throttle/vcfs
        default: []
      gqcutoff: gqcutoff
      genomebed: genomebed
      ref: ref
      nonref: nonref
    out: [fas]

  gvcf2fasta_splitvcf-imputation-wf:
    run: subworkflows/scatter/gvcf2fasta/gvcf2fasta_splitvcf-imputation-wf.cwl
    when: $(inputs.splitvcfdir !== null && inputs.chrs !== null && inputs.refsdir !== null && inputs.mapsdir !== null && inputs.panelcallbed !== null && inputs.panelnocallbed !== null)
    scatter: [sampleid, splitvcfdir]
    scatterMethod: dotproduct
    in:
      sampleid: 
        source: sampleids
        default: []
      splitvcfdir: 
        source: splitvcfdirs
        default: []
      gqcutoff: gqcutoff
      genomebed: genomebed
      ref: ref
      chrs: chrs
      refsdir: refsdir
      mapsdir: mapsdir
      panelnocallbed: panelnocallbed
      panelcallbed: panelcallbed
    out: [fas]

  gvcf2fasta_splitvcf-wf:
    run: subworkflows/scatter/gvcf2fasta/gvcf2fasta_splitvcf-wf.cwl
    when: $(inputs.split && inputs.chrs === null)
    scatter: [sampleid, splitvcfdir]
    scatterMethod: dotproduct
    in:
      sampleid: 
        source: get_sample_ids/sampleids
        default: []
      splitvcfdir: 
        source: splitvcfdirs
        default: []
      gqcutoff: gqcutoff
      genomebed: genomebed
      ref: ref
      split: split
      chrs: chrs
    out: [fas]

  gvcf2fasta_splitvcftar-wf:
    run: subworkflows/scatter/gvcf2fasta/gvcf2fasta_splitvcftar-wf.cwl
    when: $(inputs.tar === true && inputs.split === true)
    scatter: [sampleid, vcftar]
    scatterMethod: dotproduct
    in:
      sampleid: 
        source: get_sample_ids/sampleids
        default: []
      vcftar: 
        source: vcf_throttle/vcfs
        default: []
      gqcutoff: gqcutoff
      genomebed: genomebed
      ref: ref
      tar: tar
      split: split
    out: [fas]
  gvcf2fasta-wf:
    run: subworkflows/scatter/gvcf2fasta/gvcf2fasta-wf.cwl
    scatter: [sampleid, vcf]
    when: $(inputs.tar !== true && inputs.split !== true && inputs.nonref !== true)
    scatterMethod: dotproduct
    in:
      sampleid: 
        source: get_sample_ids/sampleids
        default: []
      vcf: 
        source: vcf_throttle/vcfs
        default: []
      genomebed: genomebed
      ref: ref
      gqcutoff: gqcutoff
      tar: tar
      split: split
      nonref: nonref
    out: [fas]

